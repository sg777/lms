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
	KeyID    string `json:"key_id"`
	Index    uint64 `json:"index"`
	Created  string `json:"created"`
}

// HSMServer manages LMS keys
type HSMServer struct {
	mu            sync.RWMutex
	keys          map[string]*LMSKey // key_id -> LMSKey
	port          int
	raftEndpoints []string           // Raft cluster endpoints
	attestationPrivKey *ecdsa.PrivateKey // EC private key for signing
	attestationPubKey  *ecdsa.PublicKey  // EC public key
}

// NewHSMServer creates a new HSM server
func NewHSMServer(port int, raftEndpoints []string) (*HSMServer, error) {
	// Load or generate attestation key pair
	privKey, pubKey, err := LoadOrGenerateAttestationKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to load/generate attestation keys: %v", err)
	}

	return &HSMServer{
		keys:              make(map[string]*LMSKey),
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
func (s *HSMServer) generateKey(keyID string) (*LMSKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If key_id not provided, generate one
	if keyID == "" {
		keyID = fmt.Sprintf("lms_key_%d", len(s.keys)+1)
	}

	// Check if key_id already exists
	if _, exists := s.keys[keyID]; exists {
		return nil, fmt.Errorf("key_id %s already exists", keyID)
	}

	// Create new key with index 0
	key := &LMSKey{
		KeyID:   keyID,
		Index:   0, // Always starts at 0
		Created: fmt.Sprintf("%d", len(s.keys)+1), // Simple timestamp
	}

	s.keys[keyID] = key
	return key, nil
}

// listKeys returns all keys
func (s *HSMServer) listKeys() []LMSKey {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]LMSKey, 0, len(s.keys))
	for _, key := range s.keys {
		keys = append(keys, *key)
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
	
	return http.ListenAndServe(addr, mux)
}

