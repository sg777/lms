package service

import (
	"encoding/json"
	"net/http"
)

// handleKeys returns all key IDs in the cluster
func (s *APIServer) handleKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, "/keys")
		return
	}

	// Get all key IDs from FSM (Phase B: use GetAllKeyIDs which returns key_id values)
	if keyIndexFSM, ok := s.fsm.(interface{ GetAllKeyIDs() []string }); ok {
		keyIDs := keyIndexFSM.GetAllKeyIDs()
		
		response := map[string]interface{}{
			"success": true,
			"keys":    keyIDs,
			"count":   len(keyIDs),
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Fallback: try to get from GetAllKeyIndices (but this returns pubkey_hash -> index, not ideal)
	allKeys := s.fsm.GetAllKeyIndices()
	
	// Extract pubkey_hashes (fallback - not ideal since we want key_ids)
	keyIDs := make([]string, 0, len(allKeys))
	for pubkeyHash := range allKeys {
		keyIDs = append(keyIDs, pubkeyHash)
	}

	response := map[string]interface{}{
		"success": true,
		"keys":    keyIDs,
		"count":   len(keyIDs),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

