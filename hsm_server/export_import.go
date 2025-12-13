package hsm_server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ExportKeyResponse represents the exported key data
type ExportKeyResponse struct {
	Success    bool   `json:"success"`
	KeyID      string `json:"key_id,omitempty"`
	PrivateKey []byte `json:"private_key"` // Serialized LMS private key
	PublicKey  []byte `json:"public_key"`  // Serialized LMS public key
	Index      uint64 `json:"index"`       // Current index
	Params     string `json:"params"`      // LMS parameters description
	Levels     int    `json:"levels"`
	LmType     []int  `json:"lm_type"`
	OtsType    []int  `json:"ots_type"`
	Created    string `json:"created"`
	Error      string `json:"error,omitempty"`
}

// ImportKeyRequest represents the import request
type ImportKeyRequest struct {
	PrivateKey string `json:"private_key"` // Base64-encoded LMS private key
	PublicKey  string `json:"public_key"`  // Base64-encoded LMS public key
	Index      uint64 `json:"index"`       // Starting index (should match exported index)
	Params     string `json:"params"`      // LMS parameters description
	Levels     int    `json:"levels"`
	LmType     []int  `json:"lm_type"`
	OtsType    []int  `json:"ots_type"`
	KeyID      string `json:"key_id,omitempty"` // Optional: new key_id for this user
	UserID     string `json:"user_id,omitempty"` // User ID from JWT (added by explorer proxy)
}

// ImportKeyResponse represents the import response
type ImportKeyResponse struct {
	Success bool   `json:"success"`
	KeyID   string `json:"key_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// handleExportKey exports a key for the authenticated user
func (s *HSMServer) handleExportKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		KeyID  string `json:"key_id"`
		UserID string `json:"user_id,omitempty"` // User ID from JWT (added by explorer proxy)
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := ExportKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.KeyID == "" {
		response := ExportKeyResponse{
			Success: false,
			Error:   "key_id is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get user_id from request or JWT token
	userID := req.UserID
	if userID == "" {
		if tokenUserID, err := getUserIdFromRequest(r); err == nil {
			userID = tokenUserID
		}
	}

	// Verify ownership and get key
	s.mu.RLock()
	key, exists := s.keys[req.KeyID]
	s.mu.RUnlock()

	if !exists {
		// Try database
		dbKey, err := s.db.GetKey(req.KeyID)
		if err != nil || dbKey == nil {
			response := ExportKeyResponse{
				Success: false,
				Error:   fmt.Sprintf("Key %s not found", req.KeyID),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(response)
			return
		}
		key = dbKey
	}

	// Verify ownership if user_id provided
	if userID != "" && key.UserID != "" && key.UserID != userID {
		response := ExportKeyResponse{
			Success: false,
			Error:   "You do not have permission to export this key",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Export the key (including private key)
	response := ExportKeyResponse{
		Success:    true,
		KeyID:      key.KeyID,
		PrivateKey: key.PrivateKey,
		PublicKey:  key.PublicKey,
		Index:      key.Index,
		Params:     key.Params,
		Levels:     key.Levels,
		LmType:     key.LmType,
		OtsType:    key.OtsType,
		Created:    key.Created,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleImportKey imports a key for the authenticated user
func (s *HSMServer) handleImportKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ImportKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := ImportKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if req.PrivateKey == "" {
		response := ImportKeyResponse{
			Success: false,
			Error:   "private_key is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.PublicKey == "" {
		response := ImportKeyResponse{
			Success: false,
			Error:   "public_key is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get user_id from request or JWT token
	userID := req.UserID
	if userID == "" {
		if tokenUserID, err := getUserIdFromRequest(r); err == nil {
			userID = tokenUserID
		}
	}

	if userID == "" {
		response := ImportKeyResponse{
			Success: false,
			Error:   "Authentication required to import key",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Generate key_id if not provided
	keyID := req.KeyID
	if keyID == "" {
		s.mu.RLock()
		keyCount := len(s.keys)
		s.mu.RUnlock()
		keyID = fmt.Sprintf("user_%s_key_%d", userID, keyCount+1)
	}

	// Check if key_id already exists for this user
	s.mu.RLock()
	existing, exists := s.keys[keyID]
	s.mu.RUnlock()
	
	if exists && existing.UserID == userID {
		response := ImportKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Key ID %s already exists for this user", keyID),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate LMS parameters
	if req.Levels == 0 || len(req.LmType) == 0 || len(req.OtsType) == 0 {
		response := ImportKeyResponse{
			Success: false,
			Error:   "LMS parameters (levels, lm_type, ots_type) are required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Decode base64 private key
	privateKeyBytes, err := base64.StdEncoding.DecodeString(req.PrivateKey)
	if err != nil {
		response := ImportKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid private_key base64 encoding: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Decode base64 public key
	publicKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		response := ImportKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid public_key base64 encoding: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create key object
	importedKey := &LMSKey{
		KeyID:      keyID,
		UserID:     userID,
		Index:      req.Index,
		PrivateKey: privateKeyBytes,
		PublicKey:  publicKeyBytes,
		Params:     req.Params,
		Levels:     req.Levels,
		LmType:     req.LmType,
		OtsType:    req.OtsType,
		Created:    "", // Will be set to current time
	}

	// Use current time for Created field
	importedKey.Created = time.Now().Format(time.RFC3339)

	// Store in database
	if err := s.db.StoreKey(keyID, importedKey); err != nil {
		response := ImportKeyResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to store imported key: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Store in memory cache
	s.mu.Lock()
	s.keys[keyID] = importedKey
	s.mu.Unlock()

	response := ImportKeyResponse{
		Success: true,
		KeyID:   keyID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteKey deletes a key for the authenticated user
func (s *HSMServer) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		KeyID  string `json:"key_id"`
		UserID string `json:"user_id,omitempty"` // User ID from JWT (added by explorer proxy)
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.KeyID == "" {
		response := map[string]interface{}{
			"success": false,
			"error":   "key_id is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get user_id from request or JWT token
	userID := req.UserID
	if userID == "" {
		if tokenUserID, err := getUserIdFromRequest(r); err == nil {
			userID = tokenUserID
		}
	}

	// Verify ownership and get key
	s.mu.RLock()
	key, exists := s.keys[req.KeyID]
	s.mu.RUnlock()

	if !exists {
		// Try database
		dbKey, err := s.db.GetKey(req.KeyID)
		if err != nil || dbKey == nil {
			response := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Key %s not found", req.KeyID),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(response)
			return
		}
		key = dbKey
	}

	// Verify ownership if user_id provided
	if userID != "" && key.UserID != "" && key.UserID != userID {
		response := map[string]interface{}{
			"success": false,
			"error":   "You do not have permission to delete this key",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Delete from database
	if err := s.db.DeleteKey(req.KeyID); err != nil {
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to delete key from database: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Remove from memory cache
	s.mu.Lock()
	delete(s.keys, req.KeyID)
	s.mu.Unlock()

	response := map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Key %s deleted successfully", req.KeyID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ComputePubkeyHash computes SHA-256 hash of the public key
func ComputePubkeyHash(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return base64.StdEncoding.EncodeToString(hash[:])
}

