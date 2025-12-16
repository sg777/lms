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
	"strconv"

	"github.com/verifiable-state-chains/lms/blockchain"
	"github.com/verifiable-state-chains/lms/fsm"
	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

// SignRequest is the request to sign a message
type SignRequest struct {
	KeyID             string `json:"key_id"`
	Message           string `json:"message"`
	UserID            string `json:"user_id,omitempty"`            // User ID from JWT token (added by explorer proxy)
	WalletAddress     string `json:"wallet_address,omitempty"`     // CHIPS wallet address for funding (set by explorer proxy)
	BlockchainEnabled bool   `json:"blockchain_enabled,omitempty"` // Whether to commit to blockchain for this key (per-key control)
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
	PublicKey string `json:"pubkey"`    // Base64-encoded LMS public key
	Index     uint64 `json:"index"`     // LMS index used for this signature
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
// fundingAddress: Optional CHIPS address to use for blockchain transaction funding
// blockchainEnabled: Whether to commit to blockchain for this specific key (per-key control)
// recordType: Record type - "create" (index 0), "sign" (next index), "sync" (sync to index), "delete" (end lifecycle)
func (s *HSMServer) commitIndexToRaft(keyID string, index uint64, previousHash string, lmsPublicKey []byte, fundingAddress string, blockchainEnabled bool, recordType string) error {
	// Compute pubkey_hash from LMS public key (Phase B: primary identifier)
	pubkeyHash := fsm.ComputePubkeyHash(lmsPublicKey) // Returns base64 string
	// Decode base64 to get raw bytes, then format as hex for API calls
	pubkeyHashBytes, err := base64.StdEncoding.DecodeString(pubkeyHash)
	if err != nil {
		return fmt.Errorf("failed to decode pubkey_hash: %v", err)
	}
	pubkeyHashHex := fmt.Sprintf("%x", pubkeyHashBytes)

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

	// Set default record_type if not provided
	if recordType == "" {
		recordType = "sign" // Default to "sign" for backward compatibility
	}

	// Create entry (without hash first, so we can compute it)
	entry := fsm.KeyIndexEntry{
		KeyID:        keyID,
		PubkeyHash:   pubkeyHash, // Phase B: primary identifier
		Index:        index,
		PreviousHash: previousHash,
		Hash:         "", // Will be computed
		Signature:    base64.StdEncoding.EncodeToString(signature),
		PublicKey:    base64.StdEncoding.EncodeToString(pubKeyBytes),
		RecordType:   recordType,
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
		"key_id":        entry.KeyID,
		"pubkey_hash":   entry.PubkeyHash, // Phase B: include pubkey_hash
		"index":         entry.Index,
		"previous_hash": entry.PreviousHash,
		"hash":          entry.Hash,
		"signature":     entry.Signature,
		"public_key":    entry.PublicKey,
		"record_type":   entry.RecordType,
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

	// Commit to blockchain if enabled
	blockchainErr := error(nil)
	// Commit to blockchain only if:
	// 1. Global blockchain is enabled (s.blockchainEnabled)
	// 2. Per-key blockchain is enabled (blockchainEnabled parameter)
	if s.blockchainEnabled && blockchainEnabled && s.blockchainClient != nil && s.blockchainIdentity != "" {
		// Commit to blockchain (per-key control)
		// Try to get wallet address from request (set by explorer proxy)
		// If available, use it explicitly for funding

		_, _, blockchainErr = s.blockchainClient.CommitLMSIndexWithPubkeyHash(
			s.blockchainIdentity,
			pubkeyHashHex,
			fmt.Sprintf("%d", index),
			fundingAddress, // Pass funding address explicitly
		)
		if blockchainErr != nil {
			log.Printf("[WARNING] Failed to commit index %d to blockchain: %v", index, blockchainErr)
		} else {
			log.Printf("[INFO] Committed index %d to blockchain (pubkey_hash=%s, funding=%s)", index, pubkeyHashHex, fundingAddress)
		}
	}

	// Handle different scenarios:
	// 1. Raft committed successfully -> success (blockchain also committed if enabled)
	// 2. Raft failed but blockchain enabled and committed -> success (fallback mode)
	// 3. Raft failed and blockchain not enabled -> error
	// 4. Raft failed and blockchain enabled but also failed -> error
	if !raftCommitted {
		if blockchainEnabled && s.blockchainEnabled {
			// Blockchain is enabled - check if it succeeded
			if blockchainErr == nil {
				// Blockchain committed successfully - allow this (fallback mode)
				log.Printf("[WARNING] Raft commit failed but blockchain commit succeeded - proceeding in fallback mode")
				return nil
			} else {
				// Both failed
				return fmt.Errorf("all endpoints failed AND blockchain commit failed: raft=%v, blockchain=%v", lastErr, blockchainErr)
			}
		} else {
			// Blockchain not enabled - Raft failure is fatal
			return fmt.Errorf("all endpoints failed: %v", lastErr)
		}
	}

	// Raft committed successfully (blockchain also committed if enabled)
	return nil
}

// syncIndexes syncs both Raft and blockchain to the same index (highest between them)
// This is used when there's a mismatch between Raft and blockchain indices
// recordType should be "sync"
func (s *HSMServer) syncIndexes(keyID string, targetIndex uint64, previousHash string, lmsPublicKey []byte, fundingAddress string, blockchainEnabled bool) error {
	log.Printf("[SYNC] Syncing key %s to index %d (previous_hash=%s)", keyID, targetIndex, previousHash)

	// Use commitIndexToRaft with record_type="sync" to commit to both Raft and blockchain
	// This will commit the target index to both systems
	return s.commitIndexToRaft(keyID, targetIndex, previousHash, lmsPublicKey, fundingAddress, blockchainEnabled, "sync")
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
	pubkeyHash := fsm.ComputePubkeyHash(lmsKey.PublicKey) // Returns base64 string
	// Decode base64 to get raw bytes, then format as hex for API calls
	pubkeyHashBytes, err := base64.StdEncoding.DecodeString(pubkeyHash)
	if err != nil {
		response := SignResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to decode pubkey_hash: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	pubkeyHashHex := fmt.Sprintf("%x", pubkeyHashBytes)

	// Step 2: Consistency check - fetch last index from both Raft and blockchain (if enabled)
	raftIndex, raftHash, raftExists, raftErr := s.queryRaftByPubkeyHash(pubkeyHash)

	var blockchainIndex uint64
	blockchainAvailable := false

	// If blockchain is enabled, fetch last index from blockchain
	if req.BlockchainEnabled && s.blockchainEnabled && s.blockchainClient != nil && s.blockchainIdentity != "" {
		blockchainIndexStr, err := s.blockchainClient.GetLatestLMSIndexByPubkeyHash(s.blockchainIdentity, pubkeyHashHex)
		if err != nil {
			// Check if error is "no commits found" (valid state) vs actual RPC error
			if err == blockchain.ErrNoCommits {
				// No commits yet - this is valid, the first commit will happen during this sign
				log.Printf("[INFO] Blockchain enabled but no commits found for key %s yet - will create first commit", req.KeyID)
				blockchainAvailable = false // No blockchain data yet
				blockchainIndex = 0
			} else {
				// Blockchain RPC error - truly unavailable
				response := SignResponse{
					Success: false,
					Error:   fmt.Sprintf("Blockchain is enabled but unavailable: %v. Please ensure CHIPS node is running.", err),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(response)
				return
			}
		} else {
			// Successfully got blockchain index
			// Parse blockchain index
			blockchainIndexUint, err := strconv.ParseUint(blockchainIndexStr, 10, 64)
			if err != nil {
				// Failed to parse blockchain index - treat as unavailable
				response := SignResponse{
					Success: false,
					Error:   fmt.Sprintf("Failed to parse blockchain index: %v", err),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(response)
				return
			}
			blockchainIndex = blockchainIndexUint
			blockchainAvailable = true
		}
	}

	// Handle Raft unavailable scenarios
	if raftErr != nil {
		if req.BlockchainEnabled && blockchainAvailable {
			// Raft unavailable but blockchain enabled and available - allow signing with blockchain only
			log.Printf("[WARNING] Raft unavailable but blockchain enabled - proceeding with blockchain only (index: %d)", blockchainIndex)
			// Use blockchain index as base
			raftIndex = blockchainIndex
			raftExists = blockchainAvailable
			raftHash = fsm.GenesisHash // Will be handled in commit logic
		} else {
			// Raft unavailable and blockchain not enabled/available - fail
			response := SignResponse{
				Success: false,
				Error:   fmt.Sprintf("Raft cluster is unavailable: %v. Cannot proceed without Raft or blockchain.", raftErr),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Consistency check: If both Raft and blockchain have data, they must match
	if raftExists && blockchainAvailable {
		if raftIndex != blockchainIndex {
			// Mismatch detected - sync both to highest index
			log.Printf("[WARNING] Index mismatch detected: Raft index=%d, Blockchain index=%d. Syncing to highest index...", raftIndex, blockchainIndex)

			highestIndex := raftIndex
			syncHash := raftHash

			if blockchainIndex > raftIndex {
				// Blockchain has higher index - need to sync Raft to blockchain
				highestIndex = blockchainIndex
				// For sync, we need the previous hash for the target index
				// Since blockchain has the higher index, we should use the hash from the entry before it
				// But we don't have that from blockchain. Instead, we'll use Raft's highest hash
				// as the previous hash for the sync commit. This brings Raft up to blockchain's level.
				syncHash = raftHash
				if syncHash == "" {
					syncHash = fsm.GenesisHash // Fallback to genesis if no hash available
				}
				log.Printf("[SYNC] Blockchain has higher index (%d > %d), syncing Raft to blockchain", blockchainIndex, raftIndex)
			} else {
				// Raft has higher index - need to sync blockchain to Raft
				highestIndex = raftIndex
				syncHash = raftHash
				if syncHash == "" {
					syncHash = fsm.GenesisHash // Fallback to genesis if no hash available
				}
				log.Printf("[SYNC] Raft has higher index (%d > %d), syncing blockchain to Raft", raftIndex, blockchainIndex)
			}

			// Perform sync: commit highest index to both Raft and blockchain with record_type="sync"
			if err := s.syncIndexes(req.KeyID, highestIndex, syncHash, lmsKey.PublicKey, req.WalletAddress, req.BlockchainEnabled); err != nil {
				response := SignResponse{
					Success: false,
					Error:   fmt.Sprintf("Failed to sync indexes: %v. Raft index=%d, Blockchain index=%d", err, raftIndex, blockchainIndex),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(response)
				return
			}

			log.Printf("[INFO] Successfully synced both Raft and blockchain to index %d", highestIndex)

			// After sync, update Raft state to reflect the synced index
			// Re-query to get the actual hash of the synced entry
			syncedIndex, syncedHash, syncedExists, err := s.queryRaftByPubkeyHash(pubkeyHash)
			if err == nil && syncedExists {
				raftIndex = syncedIndex
				raftHash = syncedHash
				raftExists = syncedExists
			} else {
				// Fallback: use the values we computed
				raftIndex = highestIndex
				raftHash = syncHash
				raftExists = true
			}
		} else {
			log.Printf("[INFO] Consistency check passed: Both Raft and blockchain at index %d", raftIndex)
		}
	}

	// Use Raft data (or blockchain if Raft unavailable)
	lastIndex := raftIndex
	lastHash := raftHash
	exists := raftExists

	var indexToUse uint64
	var previousHash string

	if !exists {
		// Step 3: Key not found, commit index 0 to Raft with genesis hash
		// This is a "create" record type (first commit for this key)
		indexToUse = 0
		previousHash = fsm.GenesisHash
		if err := s.commitIndexToRaft(req.KeyID, indexToUse, previousHash, lmsKey.PublicKey, req.WalletAddress, req.BlockchainEnabled, "create"); err != nil {
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

		// Step 3: Commit the new index (this is a "sign" record type)
		if err := s.commitIndexToRaft(req.KeyID, indexToUse, previousHash, lmsKey.PublicKey, req.WalletAddress, req.BlockchainEnabled, "sign"); err != nil {
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
