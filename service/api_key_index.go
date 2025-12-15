package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/verifiable-state-chains/lms/fsm"
)

// KeyIndexFSMInterface defines interface for key index FSM
type KeyIndexFSMInterface interface {
	GetKeyIndex(keyID string) (uint64, bool)
	GetAllKeyIndices() map[string]uint64
}

// handleKeyIndex handles requests for key_id's last index or full chain
// URL format: /key/<key_id>/index or /key/<key_id>/chain
func (s *APIServer) handleKeyIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, r.URL.Path)
		return
	}

	// Extract key_id and endpoint type from path: /key/<key_id>/index or /key/<key_id>/chain
	path := strings.TrimPrefix(r.URL.Path, "/key/")
	path = strings.Trim(path, "/") // Remove leading/trailing slashes

	var keyID string
	var endpoint string

	// Check for /chain or /index suffix
	if strings.HasSuffix(path, "/chain") {
		keyID = strings.TrimSuffix(path, "/chain")
		endpoint = "chain"
	} else if strings.HasSuffix(path, "/index") {
		keyID = strings.TrimSuffix(path, "/index")
		endpoint = "index"
	} else if path == "chain" || path == "index" {
		// Edge case: /key/chain or /key/index (no key_id)
		keyID = ""
		endpoint = path
	} else {
		// No endpoint specified, default to index
		keyID = path
		endpoint = "index"
	}

	// Clean up keyID (remove trailing slashes)
	keyID = strings.Trim(keyID, "/")

	if keyID == "" {
		response := map[string]interface{}{
			"success": false,
			"error":   "key_id is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle chain endpoint
	if endpoint == "chain" {
		// Build chain from Raft log entries (works even for entries committed before keyEntries storage was added)
		chainEntries, verification := s.buildChainFromRaftLog(keyID)

		if len(chainEntries) == 0 {
			response := map[string]interface{}{
				"success": true,
				"key_id":  keyID,
				"exists":  false,
				"chain":   []interface{}{},
				"count":   0,
				"message": "key_id not found",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		response := map[string]interface{}{
			"success":      true,
			"key_id":       keyID,
			"exists":       true,
			"chain":        chainEntries,
			"count":        len(chainEntries),
			"verification": verification,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle index endpoint (existing behavior)
	// Get key index and hash from FSM
	index, exists := s.fsm.GetKeyIndex(keyID)

	response := map[string]interface{}{
		"success": true,
		"key_id":  keyID,
		"exists":  exists,
	}

	if exists {
		response["index"] = index
		// Try to get hash if FSM supports it (for hash chain)
		if hashFSM, ok := s.fsm.(interface{ GetKeyHash(string) (string, bool) }); ok {
			if hash, hashExists := hashFSM.GetKeyHash(keyID); hashExists {
				response["hash"] = hash
			}
		}
	} else {
		response["index"] = nil
		response["hash"] = nil
		response["message"] = "key_id not found"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// buildChainFromRaftLog builds the full hash chain for a key_id by querying through FSM's stored entries first,
// then falling back to querying log entries if needed
// Returns the chain entries and verification results
func (s *APIServer) buildChainFromRaftLog(keyID string) ([]map[string]interface{}, *ChainVerification) {
	chainEntries := make([]map[string]interface{}, 0)
	verification := &ChainVerification{
		Valid:      true,
		Error:      "",
		BreakIndex: -1,
	}

	// First try to get from FSM's stored entries (if available)
	if chainFSM, ok := s.fsm.(interface {
		GetKeyChain(string) ([]*fsm.KeyIndexEntry, bool)
	}); ok {
		entries, exists := chainFSM.GetKeyChain(keyID)
		if exists && len(entries) > 0 {
			// Convert to response format and verify chain integrity
			// Note: "hash" is the CURRENT hash of THIS entry (computed on all fields except hash itself)
			// This hash becomes the "previous_hash" for the next entry in the chain

			// Verify chain integrity
			verification = s.verifyChainIntegrity(entries)

			for i, entry := range entries {
				entryMap := map[string]interface{}{
					"key_id":        entry.KeyID,
					"pubkey_hash":   entry.PubkeyHash, // Phase B: primary identifier
					"index":         entry.Index,
					"previous_hash": entry.PreviousHash, // Hash from previous entry (or genesis)
					"hash":          entry.Hash,         // Hash of THIS entry (will be previous_hash for next)
					"signature":     entry.Signature,
					"public_key":    entry.PublicKey,
				}

				// Add verification status for this entry
				if i == verification.BreakIndex {
					entryMap["chain_broken"] = true
					entryMap["chain_error"] = verification.Error
				} else if i > 0 {
					// Verify this entry's previous_hash matches previous entry's hash
					prevEntry := entries[i-1]
					if entry.PreviousHash == prevEntry.Hash {
						entryMap["chain_valid"] = true
					} else {
						entryMap["chain_broken"] = true
						entryMap["chain_error"] = fmt.Sprintf("previous_hash mismatch: expected %s, got %s", prevEntry.Hash, entry.PreviousHash)
					}
				} else {
					// First entry - verify it uses genesis hash
					if entry.PreviousHash == fsm.GenesisHash {
						entryMap["chain_valid"] = true
						entryMap["is_genesis"] = true
					} else {
						entryMap["chain_broken"] = true
						entryMap["chain_error"] = fmt.Sprintf("first entry should have genesis hash, got %s", entry.PreviousHash)
					}
				}

				chainEntries = append(chainEntries, entryMap)
			}
			return chainEntries, verification
		}
	}

	// Fallback: Since entries might have been committed before keyEntries storage was added,
	// and Raft replays logs on startup (calling Apply), they should be in keyEntries now.
	// If not found, return empty (entries don't exist or weren't KeyIndexEntry types)

	return chainEntries, verification
}

// ChainVerification represents the result of chain integrity verification
type ChainVerification struct {
	Valid      bool   `json:"valid"`
	Error      string `json:"error,omitempty"`
	BreakIndex int    `json:"break_index,omitempty"` // Index of entry where chain breaks (-1 if no break)
}

// verifyChainIntegrity verifies the integrity of a hash chain
func (s *APIServer) verifyChainIntegrity(entries []*fsm.KeyIndexEntry) *ChainVerification {
	verification := &ChainVerification{
		Valid:      true,
		BreakIndex: -1,
	}

	if len(entries) == 0 {
		verification.Valid = false
		verification.Error = "chain is empty"
		return verification
	}

	// Verify first entry uses genesis hash
	if entries[0].PreviousHash != fsm.GenesisHash {
		verification.Valid = false
		verification.Error = fmt.Sprintf("first entry previous_hash mismatch: expected %s (genesis), got %s", fsm.GenesisHash, entries[0].PreviousHash)
		verification.BreakIndex = 0
		return verification
	}

	// Verify each entry's hash computation
	for i, entry := range entries {
		computedHash, err := entry.ComputeHash()
		if err != nil {
			verification.Valid = false
			verification.Error = fmt.Sprintf("entry %d: failed to compute hash: %v", i, err)
			verification.BreakIndex = i
			return verification
		}

		if entry.Hash != computedHash {
			verification.Valid = false
			verification.Error = fmt.Sprintf("entry %d: hash mismatch: expected %s, got %s", i, computedHash, entry.Hash)
			verification.BreakIndex = i
			return verification
		}
	}

	// Verify chain links (each entry's previous_hash matches previous entry's hash)
	for i := 1; i < len(entries); i++ {
		prevEntry := entries[i-1]
		currentEntry := entries[i]

		if currentEntry.PreviousHash != prevEntry.Hash {
			verification.Valid = false
			verification.Error = fmt.Sprintf("chain broken at entry %d: previous_hash %s does not match previous entry's hash %s", i, currentEntry.PreviousHash, prevEntry.Hash)
			verification.BreakIndex = i
			return verification
		}

		// Verify index is monotonic
		if currentEntry.Index <= prevEntry.Index {
			verification.Valid = false
			verification.Error = fmt.Sprintf("chain broken at entry %d: index %d is not greater than previous index %d", i, currentEntry.Index, prevEntry.Index)
			verification.BreakIndex = i
			return verification
		}
	}

	return verification
}

// CommitIndexRequest is the request to commit an index for a key_id
type CommitIndexRequest struct {
	KeyID        string `json:"key_id"`
	PubkeyHash   string `json:"pubkey_hash"` // Phase B: primary identifier
	Index        uint64 `json:"index"`
	PreviousHash string `json:"previous_hash"` // SHA-256 hash of previous entry (genesis: all 0's)
	Hash         string `json:"hash"`          // SHA-256 hash of this entry
	Signature    string `json:"signature"`     // Base64 encoded EC signature
	PublicKey    string `json:"public_key"`    // Base64 encoded EC public key
	RecordType   string `json:"record_type"`   // Record type: "create", "sign", "sync", "delete"
}

// CommitIndexResponse is the response from committing an index
type CommitIndexResponse struct {
	Success   bool   `json:"success"`
	KeyID     string `json:"key_id,omitempty"`
	Index     uint64 `json:"index,omitempty"`
	Committed bool   `json:"committed"`
	Error     string `json:"error,omitempty"`
}

// handleCommitIndex handles requests to commit an index for a key_id
func (s *APIServer) handleCommitIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, "/commit_index")
		return
	}

	var req CommitIndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := CommitIndexResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate request format - this service only handles LMS index-related messages
	if req.KeyID == "" {
		response := CommitIndexResponse{
			Success: false,
			Error:   "key_id is required for LMS index commitment",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate signature and public key are provided
	if req.Signature == "" {
		response := CommitIndexResponse{
			Success: false,
			Error:   "signature is required (only HSM server with attestation key can commit)",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.PublicKey == "" {
		response := CommitIndexResponse{
			Success: false,
			Error:   "public_key is required (only HSM server with attestation key can commit)",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate pubkey_hash is present (Phase B requirement)
	if req.PubkeyHash == "" {
		response := CommitIndexResponse{
			Success: false,
			Error:   "pubkey_hash is required (Phase B: primary identifier)",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create KeyIndexEntry for validation
	// Set default record_type if not provided (backward compatibility)
	recordType := req.RecordType
	if recordType == "" {
		recordType = "sign" // Default to "sign" for backward compatibility
	}

	entry := fsm.KeyIndexEntry{
		KeyID:        req.KeyID,
		PubkeyHash:   req.PubkeyHash, // Phase B: primary identifier
		Index:        req.Index,
		PreviousHash: req.PreviousHash,
		Hash:         req.Hash,
		Signature:    req.Signature,
		PublicKey:    req.PublicKey,
		RecordType:   recordType,
	}

	// Validate message format: should be "key_id:index" format
	expectedData := fmt.Sprintf("%s:%d", req.KeyID, req.Index)
	if expectedData == "" {
		response := CommitIndexResponse{
			Success: false,
			Error:   "invalid message format for LMS index commitment",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify signature BEFORE applying to Raft (early rejection)
	// This ensures only HSM server with correct attestation key can commit
	if err := fsm.VerifyCommitSignature(&entry); err != nil {
		response := CommitIndexResponse{
			Success: false,
			Error:   fmt.Sprintf("signature verification failed: %v (only HSM server with attestation key can commit)", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Serialize entry
	entryData, err := json.Marshal(entry)
	if err != nil {
		response := CommitIndexResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to serialize entry: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Apply to Raft
	future := s.raft.Apply(entryData, s.config.RequestTimeout)
	if err := future.Error(); err != nil {
		response := CommitIndexResponse{
			Success: false,
			Error:   fmt.Sprintf("Raft apply failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if it was successful
	result := future.Response()
	if resultStr, ok := result.(string); ok && strings.HasPrefix(resultStr, "Error:") {
		response := CommitIndexResponse{
			Success: false,
			Error:   resultStr,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := CommitIndexResponse{
		Success:   true,
		KeyID:     req.KeyID,
		Index:     req.Index,
		Committed: true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
