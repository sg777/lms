package service

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/verifiable-state-chains/lms/fsm"
)

// handlePubkeyHashIndex handles requests for pubkey_hash's last index or full chain (Phase B)
func (s *APIServer) handlePubkeyHashIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, r.URL.Path)
		return
	}

	// Parse path: /pubkey_hash/<pubkey_hash>/index or /pubkey_hash/<pubkey_hash>/chain
	path := strings.TrimPrefix(r.URL.Path, "/pubkey_hash/")
	
	var pubkeyHash string
	var endpoint string
	
	if strings.HasSuffix(path, "/chain") {
		pubkeyHash = strings.TrimSuffix(path, "/chain")
		endpoint = "chain"
	} else if strings.HasSuffix(path, "/index") {
		pubkeyHash = strings.TrimSuffix(path, "/index")
		endpoint = "index"
	} else if path == "chain" || path == "index" {
		// Edge case: /pubkey_hash/chain or /pubkey_hash/index (no pubkey_hash)
		pubkeyHash = ""
		endpoint = path
	} else {
		// No endpoint specified, default to index
		pubkeyHash = path
		endpoint = "index"
	}
	
	// Clean up pubkeyHash (remove trailing slashes)
	pubkeyHash = strings.Trim(pubkeyHash, "/")

	if pubkeyHash == "" {
		response := map[string]interface{}{
			"success": false,
			"error":   "pubkey_hash is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle chain endpoint
	if endpoint == "chain" {
		// Get chain by pubkey_hash
		if chainFSM, ok := s.fsm.(interface{ GetChainByPubkeyHash(string) ([]*fsm.KeyIndexEntry, bool) }); ok {
			entries, exists := chainFSM.GetChainByPubkeyHash(pubkeyHash)
			if exists && len(entries) > 0 {
				// Convert to response format and verify chain integrity
				verification := s.verifyChainIntegrity(entries)
				
				chainEntries := make([]map[string]interface{}, len(entries))
				for i, entry := range entries {
					entryMap := map[string]interface{}{
						"key_id":        entry.KeyID,
						"pubkey_hash":   entry.PubkeyHash,
						"index":         entry.Index,
						"previous_hash": entry.PreviousHash,
						"hash":          entry.Hash,
						"signature":     entry.Signature,
						"public_key":    entry.PublicKey,
					}
					
					// Add verification status for this entry
					if i == verification.BreakIndex {
						entryMap["chain_broken"] = true
					}
					
					// Mark genesis entry
					if entry.PreviousHash == fsm.GenesisHash {
						entryMap["is_genesis"] = true
					}
					
					chainEntries[i] = entryMap
				}
				
				response := map[string]interface{}{
					"success":      true,
					"pubkey_hash":  pubkeyHash,
					"exists":       true,
					"chain":        chainEntries,
					"count":        len(chainEntries),
					"verification": verification,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}
		
		// Chain not found
		response := map[string]interface{}{
			"success":     true,
			"pubkey_hash": pubkeyHash,
			"exists":      false,
			"chain":       []interface{}{},
			"count":       0,
			"message":     "pubkey_hash not found",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Handle index endpoint
	// Get index and hash by pubkey_hash
	if indexFSM, ok := s.fsm.(interface{ GetIndexAndHashByPubkeyHash(string) (uint64, string, bool) }); ok {
		index, hash, exists := indexFSM.GetIndexAndHashByPubkeyHash(pubkeyHash)
		
		response := map[string]interface{}{
			"success":     true,
			"pubkey_hash": pubkeyHash,
			"exists":      exists,
		}
		
		if exists {
			response["index"] = index
			response["hash"] = hash
		} else {
			response["index"] = nil
			response["hash"] = nil
			response["message"] = "pubkey_hash not found"
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Fallback: FSM doesn't support pubkey_hash lookup
	response := map[string]interface{}{
		"success": false,
		"error":   "pubkey_hash lookup not supported by FSM",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(response)
}

