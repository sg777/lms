package service

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/verifiable-state-chains/lms/fsm"
)

// handleAllEntries handles requests for all entries from Raft
// URL format: /all_entries?limit=N
func (s *APIServer) handleAllEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, r.URL.Path)
		return
	}

	// Get limit from query parameter (default 10)
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 1000 {
				limit = 1000 // Max limit
			}
		}
	}

	// Get all entries from FSM (ordered by Raft log index, newest first)
	if allEntriesFSM, ok := s.fsm.(interface {
		GetAllEntries(int) []struct {
			Entry     *fsm.KeyIndexEntry
			RaftIndex uint64
		}
	}); ok {
		entriesWithIndex := allEntriesFSM.GetAllEntries(limit)

		// Convert to response format
		entries := make([]map[string]interface{}, len(entriesWithIndex))
		for i, item := range entriesWithIndex {
			entries[i] = map[string]interface{}{
				"key_id":        item.Entry.KeyID,
				"pubkey_hash":   item.Entry.PubkeyHash,
				"index":         item.Entry.Index,
				"previous_hash": item.Entry.PreviousHash,
				"hash":          item.Entry.Hash,
				"signature":     item.Entry.Signature,
				"public_key":    item.Entry.PublicKey,
				"record_type":   item.Entry.RecordType,
				"raft_index":    item.RaftIndex,
			}
		}

		response := map[string]interface{}{
			"success": true,
			"entries": entries,
			"count":   len(entries),
			"limit":   limit,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// FSM doesn't support GetAllEntries
	response := map[string]interface{}{
		"success": false,
		"error":   "GetAllEntries not supported by FSM",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(response)
}

