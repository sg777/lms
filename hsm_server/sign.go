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

	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

// SignRequest is the request to sign a message
type SignRequest struct {
	KeyID  string `json:"key_id"`
	Message string `json:"message"`
}

// SignResponse is the response from signing
type SignResponse struct {
	Success   bool   `json:"success"`
	KeyID     string `json:"key_id,omitempty"`
	Index     uint64 `json:"index,omitempty"`
	Signature string `json:"signature"` // Empty for now
	Error     string `json:"error,omitempty"`
}

// queryRaftForKeyIndex queries Raft cluster for key_id's last index
func (s *HSMServer) queryRaftForKeyIndex(keyID string) (uint64, bool, error) {
	var lastErr error
	
	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/key/%s/index", endpoint, keyID)
		
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
			return 0, false, nil // Key not found
		}
		
		index, ok := response["index"].(float64) // JSON numbers are float64
		if !ok {
			return 0, false, fmt.Errorf("invalid index in response")
		}
		
		return uint64(index), true, nil
	}
	
	return 0, false, fmt.Errorf("all endpoints failed: %v", lastErr)
}

// commitIndexToRaft commits an index to Raft cluster with EC signature
func (s *HSMServer) commitIndexToRaft(keyID string, index uint64) error {
	// Create data to sign: key_id:index
	data := fmt.Sprintf("%s:%d", keyID, index)
	hash := sha256.Sum256([]byte(data))
	
	// Debug: log the data being signed (remove in production if needed)
	fmt.Printf("[DEBUG] Signing data: %s, hash: %x\n", data, hash)
	
	// Sign with EC private key (ASN.1 format)
	signature, err := ecdsa.SignASN1(rand.Reader, s.attestationPrivKey, hash[:])
	if err != nil {
		return fmt.Errorf("failed to sign: %v", err)
	}
	
	// Encode public key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(s.attestationPubKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %v", err)
	}
	
	// Create commit request
	sigBase64 := base64.StdEncoding.EncodeToString(signature)
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)
	
	fmt.Printf("[DEBUG] Signature (base64): %s\n", sigBase64)
	fmt.Printf("[DEBUG] Public key (base64): %s\n", pubKeyBase64)
	
	commitReq := map[string]interface{}{
		"key_id":     keyID,
		"index":      index,
		"signature":  sigBase64,
		"public_key": pubKeyBase64,
	}
	
	reqBody, err := json.Marshal(commitReq)
	fmt.Printf("[DEBUG] Request body length: %d bytes\n", len(reqBody))
	if err != nil {
		return fmt.Errorf("failed to marshal request: %v", err)
	}
	
	var lastErr error
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
		
		return nil // Success
	}
	
	return fmt.Errorf("all endpoints failed: %v", lastErr)
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

	// Step 1: Query Raft cluster for key_id's last index
	lastIndex, exists, err := s.queryRaftForKeyIndex(req.KeyID)
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
	if !exists {
		// Step 2: Key not found, commit index 0 to Raft
		indexToUse = 0
		if err := s.commitIndexToRaft(req.KeyID, indexToUse); err != nil {
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
		// Key exists, use next index
		indexToUse = lastIndex + 1
		// Commit the new index
		if err := s.commitIndexToRaft(req.KeyID, indexToUse); err != nil {
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

	// Step 3: Load LMS key from database
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

	// Encode signature as base64 for JSON response
	signature := base64.StdEncoding.EncodeToString(signatureBytes)
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

	response := SignResponse{
		Success:   true,
		KeyID:     req.KeyID,
		Index:     indexToUse,
		Signature: signature, // TODO: Replace with actual LMS signature
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

