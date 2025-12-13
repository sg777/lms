package explorer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"
)

// getRecentCommits fetches recent commits from Raft cluster
func (s *ExplorerServer) getRecentCommits(limit int) ([]CommitInfo, error) {
	// Check cache first
	s.cacheMu.RLock()
	if time.Since(s.cacheLastUpdated) < s.cacheTTL && len(s.recentCommits) > 0 {
		commits := s.recentCommits
		s.cacheMu.RUnlock()
		
		// Return limited results
		if limit < len(commits) {
			return commits[:limit], nil
		}
		return commits, nil
	}
	s.cacheMu.RUnlock()

	// Fetch from Raft cluster
	allCommits := make([]CommitInfo, 0)
	
	// Get all keys first
	keys, err := s.getAllKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %v", err)
	}

	// For each key, get its chain and extract commits
	for _, keyID := range keys {
		chain, err := s.getChain(keyID)
		if err != nil {
			continue // Skip keys that fail
		}

		for _, entry := range chain.Entries {
			commit := CommitInfo{
				KeyID:        entry.KeyID,
				Index:        entry.Index,
				Hash:         entry.Hash,
				PreviousHash: entry.PreviousHash,
				Timestamp:    time.Now(), // Raft doesn't provide timestamp, use current
			}
			allCommits = append(allCommits, commit)
		}
	}

	// Sort by RaftIndex (most recent first) - approximate
	sort.Slice(allCommits, func(i, j int) bool {
		return allCommits[i].RaftIndex > allCommits[j].RaftIndex
	})

	// Update cache
	s.cacheMu.Lock()
	s.recentCommits = allCommits
	s.cacheLastUpdated = time.Now()
	s.cacheMu.Unlock()

	// Return limited results
	if limit < len(allCommits) {
		return allCommits[:limit], nil
	}
	return allCommits, nil
}

// getAllKeys gets all key IDs from the cluster
func (s *ExplorerServer) getAllKeys() ([]string, error) {
	var lastErr error
	
	// Try the new /keys endpoint first
	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/keys", endpoint)
		resp, err := s.client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			lastErr = err
			continue
		}

		success, _ := response["success"].(bool)
		if !success {
			continue
		}

		// Extract keys array
		if keysArray, ok := response["keys"].([]interface{}); ok {
			keys := make([]string, 0, len(keysArray))
			for _, key := range keysArray {
				if keyStr, ok := key.(string); ok {
					keys = append(keys, keyStr)
				}
			}
			if len(keys) > 0 {
				return keys, nil
			}
		}
	}

	// Fallback: return empty list if /keys endpoint is not available or returns empty
	if lastErr != nil {
		return []string{}, nil // Return empty list instead of error to allow stats to work
	}

	return []string{}, nil
}

// search performs a search by key_id, hash, or index
func (s *ExplorerServer) search(query string) (*SearchResponse, error) {
	// Try searching as key_id first (get full chain)
	chain, err := s.getChain(query)
	if err == nil && chain != nil && chain.Count > 0 {
		return &SearchResponse{
			Type:  "key_id",
			KeyID: query,
			Chain: chain,
		}, nil
	}

	// Try searching by hash across all keys
	// If query looks like a hash (base64, reasonable length), search by hash
	if len(query) >= 32 && len(query) <= 100 {
		entry, keyID, err := s.findEntryByHash(query)
		if err == nil && entry != nil {
			return &SearchResponse{
				Type:  "hash",
				KeyID: keyID,
				Entry: entry,
			}, nil
		}
	}

	return &SearchResponse{
		Type:    "not_found",
		Message: fmt.Sprintf("No results found for: %s", query),
	}, nil
}

// findEntryByHash searches for an entry by hash across all keys
func (s *ExplorerServer) findEntryByHash(hash string) (*ChainEntry, string, error) {
	// Get all keys first
	keys, err := s.getAllKeys()
	if err != nil {
		// If we can't get all keys, try a few common key patterns
		keys = []string{"lms_key_1", "lms_key_2", "lms_key_3"}
	}

	// Search each key's chain for the hash
	for _, keyID := range keys {
		chain, err := s.getChain(keyID)
		if err != nil {
			continue
		}

		// Check each entry's hash
		for _, entry := range chain.Entries {
			if entry.Hash == hash || entry.PreviousHash == hash {
				return &entry, keyID, nil
			}
		}
	}

	return nil, "", fmt.Errorf("hash not found")
}

// getChain fetches the full chain for a key_id
func (s *ExplorerServer) getChain(keyID string) (*ChainResponse, error) {
	var lastErr error

	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/key/%s/chain", endpoint, keyID)
		resp, err := s.client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			continue
		}

		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			lastErr = err
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
			return nil, fmt.Errorf("key_id not found")
		}

		// Parse chain entries
		chainArray, ok := response["chain"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid chain format")
		}

		entries := make([]ChainEntry, 0, len(chainArray))
		for _, entryData := range chainArray {
			entryMap, ok := entryData.(map[string]interface{})
			if !ok {
				continue
			}

			entry := ChainEntry{
				KeyID:        getString(entryMap, "key_id"),
				PreviousHash: getString(entryMap, "previous_hash"),
				Hash:         getString(entryMap, "hash"),
				Signature:    getString(entryMap, "signature"),
				PublicKey:    getString(entryMap, "public_key"),
			}

			if idx, ok := entryMap["index"].(float64); ok {
				entry.Index = uint64(idx)
			}

			if genesis, ok := entryMap["is_genesis"].(bool); ok {
				entry.IsGenesis = genesis
			} else if entry.PreviousHash == "0000000000000000000000000000000000000000000000000000000000000000" {
				entry.IsGenesis = true
			}

			if valid, ok := entryMap["chain_valid"].(bool); ok {
				entry.ChainValid = valid
			}

			if err, ok := entryMap["chain_error"].(string); ok {
				entry.ChainError = err
			}

			entries = append(entries, entry)
		}

		// Parse verification
		chainResp := &ChainResponse{
			KeyID:   keyID,
			Count:   len(entries),
			Entries: entries,
			Valid:   true,
		}

		if verification, ok := response["verification"].(map[string]interface{}); ok {
			if valid, ok := verification["valid"].(bool); ok {
				chainResp.Valid = valid
			}
			if err, ok := verification["error"].(string); ok {
				chainResp.Error = err
			}
			if breakIdx, ok := verification["break_index"].(float64); ok {
				chainResp.BreakIndex = int(breakIdx)
			}
		}

		return chainResp, nil
	}

	return nil, fmt.Errorf("all endpoints failed: %v", lastErr)
}

// getStats returns overall statistics
func (s *ExplorerServer) getStats() (*StatsResponse, error) {
	stats := &StatsResponse{}

	// Get recent commits to calculate stats
	commits, err := s.getRecentCommits(10000) // Get a large number
	if err != nil {
		return nil, err
	}

	stats.TotalCommits = len(commits)

	// Count unique keys
	keySet := make(map[string]bool)
	for _, commit := range commits {
		keySet[commit.KeyID] = true
	}
	stats.TotalKeys = len(keySet)

	// Count valid/broken chains
	// This requires checking each key's chain
	validChains := 0
	brokenChains := 0
	for keyID := range keySet {
		chain, err := s.getChain(keyID)
		if err != nil {
			continue
		}
		if chain.Valid {
			validChains++
		} else {
			brokenChains++
		}
	}

	stats.ValidChains = validChains
	stats.BrokenChains = brokenChains

	if len(commits) > 0 {
		stats.LastCommit = commits[0].Timestamp
	}

	return stats, nil
}

// Helper function to safely get string from map
func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

