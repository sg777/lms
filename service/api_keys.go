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

	// Get all key indices from FSM
	allKeys := s.fsm.GetAllKeyIndices()

	// Extract just the key IDs (keys of the map)
	keyIDs := make([]string, 0, len(allKeys))
	for keyID := range allKeys {
		keyIDs = append(keyIDs, keyID)
	}

	response := map[string]interface{}{
		"success": true,
		"keys":    keyIDs,
		"count":   len(keyIDs),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

