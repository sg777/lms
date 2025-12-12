package hsm_server

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
)

// LMSKey represents an LMS key managed by the HSM server
type LMSKey struct {
	KeyID      string    `json:"key_id"`
	Index      uint64    `json:"index"`
	Created    string    `json:"created"`
	PrivateKey []byte    `json:"private_key,omitempty"` // Serialized LMS private key (not sent to clients)
	PublicKey  []byte    `json:"public_key,omitempty"`  // Serialized LMS public key
	Params     string    `json:"params,omitempty"`      // LMS parameters (e.g., "LMS_SHA256_M32_H5")
}

// HSMServer manages LMS keys
type HSMServer struct {
	mu                sync.RWMutex
	keys              map[string]*LMSKey // key_id -> LMSKey (in-memory cache)
	db                *KeyDB              // Persistent database
	port              int
	raftEndpoints     []string            // Raft cluster endpoints
	attestationPrivKey *ecdsa.PrivateKey  // EC private key for signing
	attestationPubKey  *ecdsa.PublicKey   // EC public key
}

// NewHSMServer creates a new HSM server
func NewHSMServer(port int, raftEndpoints []string) (*HSMServer, error) {
	// Load attestation key pair (must be generated with OpenSSL)
	privKey, pubKey, err := LoadAttestationKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to load/generate attestation keys: %v", err)
	}

	// Open persistent database
	db, err := NewKeyDB(dbFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open key database: %v", err)
	}

	// Load all keys from DB into memory cache
	keys, err := db.GetAllKeys()
	if err != nil {
		// If error, continue with empty cache (might be first run)
		keys = []*LMSKey{}
	}

	keyMap := make(map[string]*LMSKey)
	for _, key := range keys {
		keyMap[key.KeyID] = key
	}

	return &HSMServer{
		keys:              keyMap,
		db:                db,
		port:              port,
		raftEndpoints:     raftEndpoints,
		attestationPrivKey: privKey,
		attestationPubKey:  pubKey,
	}, nil
}

// GenerateKeyRequest is the request to generate a new LMS key
type GenerateKeyRequest struct {
	KeyID string `json:"key_id,omitempty"` // Optional, server generates if not provided
}

// GenerateKeyResponse is the response from generating a key
type GenerateKeyResponse struct {
	Success bool   `json:"success"`
	KeyID   string `json:"key_id"`
	Index   uint64 `json:"index"`
	Error   string `json:"error,omitempty"`
}

// ListKeysResponse is the response for listing keys
type ListKeysResponse struct {
	Success bool     `json:"success"`
	Keys    []LMSKey `json:"keys"`
	Count   int      `json:"count"`
	Error   string   `json:"error,omitempty"`
}

// generateKey generates a new LMS key
// TODO: This will be updated to generate actual LMS keys using hash-sigs library
func (s *HSMServer) generateKey(keyID string) (*LMSKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If key_id not provided, generate one
	if keyID == "" {
		keyID = fmt.Sprintf("lms_key_%d", len(s.keys)+1)
	}

	// Check if key_id already exists (check both cache and DB)
	if _, exists := s.keys[keyID]; exists {
		return nil, fmt.Errorf("key_id %s already exists", keyID)
	}

	// TODO: Generate actual LMS key pair using hash-sigs library
	// For now, create placeholder key
	key := &LMSKey{
		KeyID:   keyID,
		Index:   0, // Always starts at 0
		Created: fmt.Sprintf("%d", len(s.keys)+1), // Simple timestamp
		Params:  "LMS_SHA256_M32_H5", // Placeholder - will be set by actual key generation
		// PrivateKey and PublicKey will be set by actual LMS key generation
	}

	// Store in database
	if err := s.db.StoreKey(keyID, key); err != nil {
		return nil, fmt.Errorf("failed to store key in database: %v", err)
	}

	// Store in memory cache
	s.keys[keyID] = key
	return key, nil
}

// listKeys returns all keys (without private keys)
func (s *HSMServer) listKeys() []LMSKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]LMSKey, 0, len(s.keys))
	for _, key := range s.keys {
		// Create a copy without private key for client response
		keyCopy := LMSKey{
			KeyID:   key.KeyID,
			Index:   key.Index,
			Created: key.Created,
			PublicKey: key.PublicKey,
			Params:  key.Params,
			// PrivateKey is intentionally omitted
		}
		keys = append(keys, keyCopy)
	}
	return keys
}

// handleGenerateKey handles key generation requests
func (s *HSMServer) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := GenerateKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	key, err := s.generateKey(req.KeyID)
	if err != nil {
		response := GenerateKeyResponse{
			Success: false,
			Error:   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := GenerateKeyResponse{
		Success: true,
		KeyID:   key.KeyID,
		Index:   key.Index,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleListKeys handles list keys requests
func (s *HSMServer) handleListKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	keys := s.listKeys()
	response := ListKeysResponse{
		Success: true,
		Keys:    keys,
		Count:   len(keys),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Start starts the HSM server
func (s *HSMServer) Start() error {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/generate_key", s.handleGenerateKey)
	mux.HandleFunc("/list_keys", s.handleListKeys)
	mux.HandleFunc("/sign", s.handleSign)
	
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("HSM Server starting on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  POST /generate_key - Generate new LMS key")
	log.Printf("  GET  /list_keys   - List all keys")
	log.Printf("  POST /sign         - Sign message with key_id")
	log.Printf("Raft endpoints: %v", s.raftEndpoints)
	log.Printf("Database: %s", dbFileName)
	
	return http.ListenAndServe(addr, mux)
}

// Close closes the HSM server and database
func (s *HSMServer) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

