package hsm_server

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

// LMSKey represents an LMS key managed by the HSM server
type LMSKey struct {
	KeyID      string    `json:"key_id"`
	UserID     string    `json:"user_id,omitempty"` // Owner of the key (optional for backward compatibility)
	Index      uint64    `json:"index"`
	Created    string    `json:"created"`
	PrivateKey []byte    `json:"private_key,omitempty"` // Serialized LMS private key (not sent to clients)
	PublicKey  []byte    `json:"public_key,omitempty"`  // Serialized LMS public key
	Params     string    `json:"params,omitempty"`      // LMS parameters description (e.g., "LMS: h=5, w=1 (max 32 signatures)")
	
	// LMS parameters needed for loading working key (stored in DB but not sent to clients)
	Levels  int   `json:"levels"`  // Number of levels
	LmType  []int `json:"lm_type"` // LMS parameter set array
	OtsType []int `json:"ots_type"` // OTS parameter set array
}

// HSMServer manages LMS keys
type HSMServer struct {
	mu                 sync.RWMutex
	keys               map[string]*LMSKey // key_id -> LMSKey (in-memory cache)
	db                 *KeyDB             // Persistent database
	port               int
	raftEndpoints      []string           // Raft cluster endpoints
	attestationPrivKey *ecdsa.PrivateKey  // EC private key for signing
	attestationPubKey  *ecdsa.PublicKey   // EC public key
	
	// Standard LMS parameters (h=5, w=1)
	defaultLevels int
	defaultLmType []int
	defaultOtsType []int
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
		keys:               keyMap,
		db:                 db,
		port:               port,
		raftEndpoints:      raftEndpoints,
		attestationPrivKey: privKey,
		attestationPubKey:  pubKey,
		// Standard parameters: h=5, w=1
		defaultLevels: 1,
		defaultLmType: []int{lms_wrapper.LMS_SHA256_M32_H5},
		defaultOtsType: []int{lms_wrapper.LMOTS_SHA256_N32_W1},
	}, nil
}

// GenerateKeyRequest is the request to generate a new LMS key
type GenerateKeyRequest struct {
	KeyID  string `json:"key_id,omitempty"`  // Optional, server generates if not provided
	UserID string `json:"user_id,omitempty"` // User ID from JWT token (added by explorer proxy)
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

// generateKey generates a new LMS key using the LMS wrapper
func (s *HSMServer) generateKey(keyID string, userID string) (*LMSKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If key_id not provided, generate one
	if keyID == "" {
		if userID != "" {
			keyID = fmt.Sprintf("user_%s_key_%d", userID, len(s.keys)+1)
		} else {
			keyID = fmt.Sprintf("lms_key_%d", len(s.keys)+1)
		}
	}

	// Check if key_id already exists for this user (check both cache and DB)
	// Only check if key belongs to same user (if userID provided)
	for _, existingKey := range s.keys {
		if existingKey.KeyID == keyID {
			// If userID provided, check ownership
			if userID != "" && existingKey.UserID != userID {
				return nil, fmt.Errorf("key_id %s already exists for another user", keyID)
			}
			// If no userID (backward compatibility), allow if key has no user
			if userID == "" && existingKey.UserID != "" {
				return nil, fmt.Errorf("key_id %s already exists for a user", keyID)
			}
			return nil, fmt.Errorf("key_id %s already exists", keyID)
		}
	}

	// Generate actual LMS key pair using hash-sigs library
	log.Printf("Generating LMS key pair for key_id: %s", keyID)
	privKey, pubKey, err := lms_wrapper.GenerateKeyPair(s.defaultLevels, s.defaultLmType, s.defaultOtsType)
	if err != nil {
		return nil, fmt.Errorf("failed to generate LMS key pair: %v", err)
	}

	// Get parameter description
	paramDesc := lms_wrapper.FormatParameterSet(s.defaultLevels, s.defaultLmType, s.defaultOtsType)
	log.Printf("Generated LMS key: %s", paramDesc)

	// Create key object
	key := &LMSKey{
		KeyID:      keyID,
		UserID:     userID, // Associate key with user
		Index:      0,      // Always starts at 0
		Created:    time.Now().Format(time.RFC3339),
		PrivateKey: privKey,
		PublicKey:  pubKey,
		Params:     paramDesc,
		Levels:     s.defaultLevels,
		LmType:     make([]int, len(s.defaultLmType)),
		OtsType:    make([]int, len(s.defaultOtsType)),
	}
	copy(key.LmType, s.defaultLmType)
	copy(key.OtsType, s.defaultOtsType)

	// Store in database
	if err := s.db.StoreKey(keyID, key); err != nil {
		return nil, fmt.Errorf("failed to store key in database: %v", err)
	}

	// Store in memory cache
	s.keys[keyID] = key
	log.Printf("Successfully generated and stored LMS key: %s (pubkey: %d bytes, privkey: %d bytes)", 
		keyID, len(pubKey), len(privKey))
	return key, nil
}

// listKeys returns all keys (without private keys)
// If userID is provided, only returns keys for that user
func (s *HSMServer) listKeys(userID string) []LMSKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]LMSKey, 0, len(s.keys))
	for _, key := range s.keys {
		// Filter by userID if provided
		if userID != "" && key.UserID != userID {
			continue
		}
		
		// Create a copy without private key for client response
		keyCopy := LMSKey{
			KeyID:    key.KeyID,
			UserID:   key.UserID,
			Index:    key.Index,
			Created:  key.Created,
			PublicKey: key.PublicKey,
			Params:   key.Params,
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

	// Get user_id from request (set by explorer proxy) or from JWT token
	userID := req.UserID
	if userID == "" {
		// Try to extract from JWT token for backward compatibility
		if tokenUserID, err := getUserIdFromRequest(r); err == nil {
			userID = tokenUserID
		}
	}

	key, err := s.generateKey(req.KeyID, userID)
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

	// Get user_id from query parameter (set by explorer proxy) or from JWT token
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		// Try to extract from JWT token
		if tokenUserID, err := getUserIdFromRequest(r); err == nil {
			userID = tokenUserID
		}
	}

	keys := s.listKeys(userID)
	response := ListKeysResponse{
		Success: true,
		Keys:    keys,
		Count:   len(keys),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteAllKeys handles requests to delete all keys
func (s *HSMServer) handleDeleteAllKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Delete all keys from database
	if err := s.db.DeleteAllKeys(); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to delete keys: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Clear in-memory cache
	s.mu.Lock()
	s.keys = make(map[string]*LMSKey)
	s.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"message": "All keys deleted successfully",
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
	mux.HandleFunc("/delete_all_keys", s.handleDeleteAllKeys)
	
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("HSM Server starting on %s", addr)
	log.Printf("Endpoints:")
	log.Printf("  POST   /generate_key   - Generate new LMS key")
	log.Printf("  GET    /list_keys      - List all keys")
	log.Printf("  POST   /sign           - Sign message with key_id")
	log.Printf("  DELETE /delete_all_keys - Delete all keys (WARNING: irreversible)")
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

