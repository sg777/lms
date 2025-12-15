package hsm_server

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/verifiable-state-chains/lms/fsm"
	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

// SignRequest is the request to sign a message
type SignRequest struct {
	KeyID   string `json:"key_id"`
	Message string `json:"message"`
	UserID  string `json:"user_id,omitempty"` // User ID from JWT token (added by explorer proxy)
}

// SignResponse is the response from signing
type SignResponse struct {
	Success   bool                 `json:"success"`
	KeyID     string               `json:"key_id,omitempty"`
	Index     uint64               `json:"index,omitempty"`
	Signature *StructuredSignature `json:"signature,omitempty"` // Structured signature with pubkey, index, signature
	Error     string               `json:"error,omitempty"`
}

// StructuredSignature represents the self-contained signature format
type StructuredSignature struct {
	PublicKey string `json:"pubkey"`   // Base64-encoded LMS public key
	Index     uint64 `json:"index"`    // LMS index used for this signature
	Signature string `json:"signature"` // Base64-encoded LMS signature
}

// queryRaftByPubkeyHash queries Raft cluster for pubkey_hash's last index and hash
func (s *HSMServer) queryRaftByPubkeyHash(pubkeyHash string) (uint64, string, bool, error) {
	var lastErr error
	
	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/pubkey_hash/%s/index", endpoint, pubkeyHash)
		
		resp, err := http.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("error from %s: status %d, body: %s", endpoint, resp.StatusCode, string(body))
			continue
		}
		
		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			lastErr = fmt.Errorf("failed to decode response: %v", err)
			continue
		}
		
		success, _ := response["success"].(bool)
		if !success {
			errorMsg, _ := response["error"].(string)
			lastErr = fmt.Errorf("query failed: %s", errorMsg)
			continue
		}
		
		exists, _ := response["exists"].(bool)
		if !exists {
			return 0, "", false, nil // Key not found
		}
		
		index, ok := response["index"].(float64) // JSON numbers are float64
		if !ok {
			return 0, "", false, fmt.Errorf("invalid index in response")
		}
		
		// Get hash if present (for hash chain)
		hash := ""
		if hashVal, ok := response["hash"].(string); ok && hashVal != "" {
			hash = hashVal
		}
		
		return uint64(index), hash, true, nil
	}
	
	return 0, "", false, fmt.Errorf("all endpoints failed: %v", lastErr)
}

// commitIndexToRaft commits an index to Raft cluster with EC signature and hash chain
// Also commits to Verus blockchain if enabled (for testing/fallback)
func (s *HSMServer) commitIndexToRaft(keyID string, index uint64, previousHash string, lmsPublicKey []byte) error {
	// Compute pubkey_hash from LMS public key (Phase B: primary identifier)
	pubkeyHash := fsm.ComputePubkeyHash(lmsPublicKey)
	pubkeyHashHex := fmt.Sprintf("%x", pubkeyHash)
	
	// Create data to sign: key_id:index (signature format unchanged for compatibility)
	data := fmt.Sprintf("%s:%d", keyID, index)
	dataHash := sha256.Sum256([]byte(data))
	
	// Debug: log the data being signed (remove in production if needed)
	fmt.Printf("[DEBUG] Signing data: %s, hash: %x\n", data, dataHash)
	
	// Sign with EC private key (ASN.1 format)
	signature, err := ecdsa.SignASN1(rand.Reader, s.attestationPrivKey, dataHash[:])
	if err != nil {
		return fmt.Errorf("failed to sign: %v", err)
	}
	
	// Encode public key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(s.attestationPubKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %v", err)
	}
	
	// Create entry (without hash first, so we can compute it)
	entry := fsm.KeyIndexEntry{
		KeyID:       keyID,
		PubkeyHash:  pubkeyHash, // Phase B: primary identifier
		Index:       index,
		PreviousHash: previousHash,
		Hash:        "", // Will be computed
		Signature:   base64.StdEncoding.EncodeToString(signature),
		PublicKey:   base64.StdEncoding.EncodeToString(pubKeyBytes),
	}
	
	// Compute hash of entry (all fields except Hash)
	computedHash, err := entry.ComputeHash()
	if err != nil {
		return fmt.Errorf("failed to compute entry hash: %v", err)
	}
	entry.Hash = computedHash
	
	fmt.Printf("[DEBUG] Entry hash: %s\n", entry.Hash)
	fmt.Printf("[DEBUG] Previous hash: %s\n", entry.PreviousHash)
	
	// Create commit request
	commitReq := map[string]interface{}{
		"key_id":       entry.KeyID,
		"pubkey_hash":  entry.PubkeyHash, // Phase B: include pubkey_hash
		"index":        entry.Index,
		"previous_hash": entry.PreviousHash,
		"hash":         entry.Hash,
		"signature":    entry.Signature,
		"public_key":   entry.PublicKey,
	}
	
	reqBody, err := json.Marshal(commitReq)
	fmt.Printf("[DEBUG] Request body length: %d bytes\n", len(reqBody))
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	
	var lastErr error
	var raftCommitted bool
	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/commit_index", endpoint)
		
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		body, _ := io.ReadAll(resp.Body)
		
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("error from %s: status %d, body: %s", endpoint, resp.StatusCode, string(body))
			continue
		}
		
		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			lastErr = fmt.Errorf("failed to decode response: %v", err)
			continue
		}
		
		success, _ := response["success"].(bool)
		if !success {
			errorMsg, _ := response["error"].(string)
			lastErr = fmt.Errorf("commit failed: %s", errorMsg)
			continue
		}
		
		committed, _ := response["committed"].(bool)
		if !committed {
			lastErr = fmt.Errorf("index not committed")
			continue
		}
		
		raftCommitted = true
		break // Success
	}
	
	// If Raft commit failed, still try blockchain (for testing/fallback)
	blockchainErr := error(nil)
	if s.blockchainEnabled && s.blockchainClient != nil && s.blockchainIdentity != "" {
		// Always commit to blockchain (for testing: dual commit)
		_, _, blockchainErr = s.blockchainClient.CommitLMSIndexWithPubkeyHash(
			s.blockchainIdentity,
			pubkeyHashHex,
			fmt.Sprintf("%d", index),
		)
		if blockchainErr != nil {
			log.Printf("[WARNING] Failed to commit index %d to blockchain: %v", index, blockchainErr)
		} else {
			log.Printf("[INFO] Committed index %d to blockchain (pubkey_hash=%s)", index, pubkeyHashHex)
		}
	}
	
	// Return error only if both Raft and blockchain failed (if blockchain is enabled)
	if !raftCommitted {
		if blockchainErr != nil && s.blockchainEnabled {
			return fmt.Errorf("all endpoints failed AND blockchain commit failed: raft=%v, blockchain=%v", lastErr, blockchainErr)
		}
		return fmt.Errorf("all endpoints failed: %v", lastErr)
	}
	
	// Raft committed successfully (blockchain also committed if enabled)
	return nil
}

// handleSign handles sign requests
func (s *HSMServer) handleSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := SignResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.KeyID == "" {
		response := SignResponse{
			Success: false,
			Error:   "key_id is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get user_id from request (set by explorer proxy) or from JWT token
	userID := req.UserID
	if userID == "" {
		// Try to extract from JWT token
		if tokenUserID, err := getUserIdFromRequest(r); err == nil {
			userID = tokenUserID
		}
	}

	// Verify key ownership if user_id is provided
	if userID != "" {
		s.mu.RLock()
		key, exists := s.keys[req.KeyID]
		s.mu.RUnlock()
		
		if !exists {
			// Try database
			dbKey, err := s.db.GetKey(req.KeyID)
			if err != nil || dbKey == nil {
				response := SignResponse{
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
		
		// Check ownership
		if key.UserID != "" && key.UserID != userID {
			response := SignResponse{
				Success: false,
				Error:   "You do not have permission to use this key",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Step 1: Load LMS key from database (need public key to compute pubkey_hash)
	lmsKey, err := s.db.GetKey(req.KeyID)
	if err != nil {
		// Key might not exist in DB yet (old keys), try memory cache
		s.mu.RLock()
		cachedKey, exists := s.keys[req.KeyID]
		s.mu.RUnlock()
		if !exists {
			response := SignResponse{
				Success: false,
				Error:   fmt.Sprintf("Key %s not found", req.KeyID),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(response)
			return
		}
		lmsKey = cachedKey
	}

	// Compute pubkey_hash from LMS public key (Phase B)
	pubkeyHash := fsm.ComputePubkeyHash(lmsKey.PublicKey)

	// Step 2: Query Raft cluster for pubkey_hash's last index and hash
	lastIndex, lastHash, exists, err := s.queryRaftByPubkeyHash(pubkeyHash)
	if err != nil {
		response := SignResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to query Raft: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	var indexToUse uint64
	var previousHash string
	
	if !exists {
		// Step 3: Key not found, commit index 0 to Raft with genesis hash
		indexToUse = 0
		previousHash = fsm.GenesisHash
		if err := s.commitIndexToRaft(req.KeyID, indexToUse, previousHash, lmsKey.PublicKey); err != nil {
			response := SignResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to commit index to Raft: %v", err),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}
	} else {
		// Key exists, use next index and previous entry's stored hash
		indexToUse = lastIndex + 1
		
		// MUST use the stored hash from previous entry - never recompute or fallback
		if lastHash == "" {
			response := SignResponse{
				Success: false,
				Error:   fmt.Sprintf("Previous entry hash not found for key_id %s (index %d). Cannot continue hash chain.", req.KeyID, lastIndex),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}
		
		// Use the actual stored hash from the previous commit
		previousHash = lastHash
		
		// Step 3: Commit the new index
		if err := s.commitIndexToRaft(req.KeyID, indexToUse, previousHash, lmsKey.PublicKey); err != nil {
			response := SignResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to commit index to Raft: %v", err),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Ensure LmType and OtsType are set (they might be missing from old keys)
	// If they're empty, use default values (h=5, w=1)
	if len(lmsKey.LmType) == 0 || len(lmsKey.OtsType) == 0 || lmsKey.Levels == 0 {
		log.Printf("Warning: Key %s missing LMS parameters, setting defaults (h=5, w=1)", req.KeyID)
		lmsKey.Levels = 1
		lmsKey.LmType = []int{lms_wrapper.LMS_SHA256_M32_H5}
		lmsKey.OtsType = []int{lms_wrapper.LMOTS_SHA256_N32_W1}
		// Update in database and cache
		if err := s.db.StoreKey(req.KeyID, lmsKey); err != nil {
			log.Printf("Warning: Failed to update key parameters in DB: %v", err)
		}
		s.mu.Lock()
		if cachedKey, exists := s.keys[req.KeyID]; exists {
			cachedKey.Levels = lmsKey.Levels
			cachedKey.LmType = lmsKey.LmType
			cachedKey.OtsType = lmsKey.OtsType
		}
		s.mu.Unlock()
	}

	// Step 4: Sign the message with LMS key
	if len(lmsKey.PrivateKey) == 0 {
		response := SignResponse{
			Success: false,
			Error:   fmt.Sprintf("Key %s has no private key (cannot sign)", req.KeyID),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Load working key from private key
	workingKey, err := lms_wrapper.LoadWorkingKey(
		lmsKey.PrivateKey,
		lmsKey.Levels,
		lmsKey.LmType,
		lmsKey.OtsType,
		0, // memory target: 0 = minimal memory
	)
	if err != nil {
		response := SignResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to load working key: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer workingKey.Free()

	// Generate LMS signature
	messageBytes := []byte(req.Message)
	signatureBytes, err := workingKey.GenerateSignature(messageBytes)
	if err != nil {
		response := SignResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to generate LMS signature: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get updated private key state (LMS is stateful)
	updatedPrivKey := workingKey.GetPrivateKey()
	if len(updatedPrivKey) == 0 {
		response := SignResponse{
			Success: false,
			Error:   "Failed to get updated private key state",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Update private key in database (stateful - key changes after each signature)
	lmsKey.PrivateKey = updatedPrivKey
	if err := s.db.StoreKey(req.KeyID, lmsKey); err != nil {
		log.Printf("Warning: Failed to update private key state in DB: %v", err)
	}

	// Also update in-memory cache
	s.mu.Lock()
	if cachedKey, exists := s.keys[req.KeyID]; exists {
		cachedKey.PrivateKey = updatedPrivKey
	}
	s.mu.Unlock()

	// Encode signature and public key as base64 for JSON response
	signatureB64 := base64.StdEncoding.EncodeToString(signatureBytes)
	publicKeyB64 := base64.StdEncoding.EncodeToString(lmsKey.PublicKey)
	
	log.Printf("[DEBUG] Generated LMS signature for key_id=%s, index=%d, signature_len=%d bytes", 
		req.KeyID, indexToUse, len(signatureBytes))
	
	// Step 5: Update index in database after signing
	newIndex := indexToUse + 1
	if err := s.db.UpdateKeyIndex(req.KeyID, newIndex); err != nil {
		// Also update in-memory cache
		s.mu.Lock()
		if key, exists := s.keys[req.KeyID]; exists {
			key.Index = newIndex
		}
		s.mu.Unlock()
	}

	// Create structured signature
	structuredSig := &StructuredSignature{
		PublicKey: publicKeyB64,
		Index:     indexToUse,
		Signature: signatureB64,
	}

	response := SignResponse{
		Success:   true,
		KeyID:     req.KeyID,
		Index:     indexToUse,
		Signature: structuredSig,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

