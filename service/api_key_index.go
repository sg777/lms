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
	
	var keyID string
	var endpoint string
	
	if strings.HasSuffix(path, "/chain") {
		keyID = strings.TrimSuffix(path, "/chain")
		endpoint = "chain"
	} else if strings.HasSuffix(path, "/index") {
		keyID = strings.TrimSuffix(path, "/index")
		endpoint = "index"
	} else {
		keyID = path
		endpoint = "index" // Default to index
	}

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
		// Try to get full chain if FSM supports it
		if chainFSM, ok := s.fsm.(interface{ GetKeyChain(string) ([]*fsm.KeyIndexEntry, bool) }); ok {
			entries, exists := chainFSM.GetKeyChain(keyID)
			if !exists {
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

			// Convert entries to JSON-serializable format
			chainEntries := make([]map[string]interface{}, len(entries))
			for i, entry := range entries {
				chainEntries[i] = map[string]interface{}{
					"key_id":       entry.KeyID,
					"index":        entry.Index,
					"previous_hash": entry.PreviousHash,
					"hash":         entry.Hash,
					"signature":    entry.Signature,
					"public_key":   entry.PublicKey,
				}
			}

			response := map[string]interface{}{
				"success": true,
				"key_id":  keyID,
				"exists":  true,
				"chain":   chainEntries,
				"count":   len(chainEntries),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		} else {
			response := map[string]interface{}{
				"success": false,
				"error":   "Chain retrieval not supported by FSM",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotImplemented)
			json.NewEncoder(w).Encode(response)
			return
		}
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

// CommitIndexRequest is the request to commit an index for a key_id
type CommitIndexRequest struct {
	KeyID       string `json:"key_id"`
	Index       uint64 `json:"index"`
	PreviousHash string `json:"previous_hash"` // SHA-256 hash of previous entry (genesis: all 0's)
	Hash        string `json:"hash"`           // SHA-256 hash of this entry
	Signature   string `json:"signature"`      // Base64 encoded EC signature
	PublicKey   string `json:"public_key"`     // Base64 encoded EC public key
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

	// Create KeyIndexEntry for validation
	entry := fsm.KeyIndexEntry{
		KeyID:       req.KeyID,
		Index:       req.Index,
		PreviousHash: req.PreviousHash,
		Hash:        req.Hash,
		Signature:   req.Signature,
		PublicKey:   req.PublicKey,
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

