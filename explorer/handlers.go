package explorer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// handleRecent returns recent commits
func (s *ExplorerServer) handleRecent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get limit from query parameter (default 50)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
		if limit > 200 {
			limit = 200 // Max limit
		}
		if limit < 1 {
			limit = 1
		}
	}

	commits, err := s.getRecentCommits(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get recent commits: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"commits": commits,
		"count":   len(commits),
	})
}

// handleSearch handles search queries (key_id, hash, or index)
func (s *ExplorerServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "Query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	// Try to determine what type of search this is
	// If it looks like a hash (base64, ~44 chars), search by hash
	// Otherwise, assume it's a key_id
	result, err := s.search(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleStats returns overall statistics
func (s *ExplorerServer) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats, err := s.getStats()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"stats":   stats,
	})
}

// handleChain returns full chain for a key_id
// URL: /api/chain/<key_id>
func (s *ExplorerServer) handleChain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract key_id from path
	path := strings.TrimPrefix(r.URL.Path, "/api/chain/")
	keyID := strings.Trim(path, "/")

	if keyID == "" {
		http.Error(w, "key_id is required", http.StatusBadRequest)
		return
	}

	chain, err := s.getChain(keyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get chain: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"chain":   chain,
	})
}

