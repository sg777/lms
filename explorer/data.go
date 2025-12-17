package explorer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// getRecentCommits fetches recent commits from Raft cluster using /all_entries endpoint
func (s *ExplorerServer) getRecentCommits(limit int) ([]CommitInfo, error) {
	// Fetch from Raft cluster using /all_entries endpoint
	allCommits := make([]CommitInfo, 0)

	var lastErr error
	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/all_entries?limit=%d", endpoint, limit+10) // Fetch extra for comparison
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

		entries, ok := response["entries"].([]interface{})
		if !ok {
			lastErr = fmt.Errorf("invalid entries format")
			continue
		}

		// Convert to CommitInfo
		for _, entryData := range entries {
			entryMap, ok := entryData.(map[string]interface{})
			if !ok {
				continue
			}

			commit := CommitInfo{
				KeyID:        getString(entryMap, "key_id"),
				PubkeyHash:   getString(entryMap, "pubkey_hash"),
				PreviousHash: getString(entryMap, "previous_hash"),
				Hash:         getString(entryMap, "hash"),
				Timestamp:    time.Now(), // Raft doesn't provide timestamp
			}

			if idx, ok := entryMap["index"].(float64); ok {
				commit.Index = uint64(idx)
			}

			if raftIdx, ok := entryMap["raft_index"].(float64); ok {
				commit.RaftIndex = uint64(raftIdx)
			}

			allCommits = append(allCommits, commit)
		}

		// Success - break out of loop
		break
	}

	if len(allCommits) == 0 && lastErr != nil {
		return nil, fmt.Errorf("failed to fetch entries from all endpoints: %v", lastErr)
	}

	// Entries are already sorted by Raft log index (newest first) from the API
	// Apply limit
	if limit > 0 && limit < len(allCommits) {
		allCommits = allCommits[:limit]
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
				PubkeyHash:   getString(entryMap, "pubkey_hash"),
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

	// Get blockchain commits to calculate accurate stats (filtered by bootstrap height)
	// This ensures stats match what's shown in the blockchain explorer
	client := newVerusClientFromEnv()
	identityName := verusIdentityName()
	bootstrapHeight := getBootstrapBlockHeight()

	// Get all commits from blockchain
	commits, err := client.QueryAttestationCommits(identityName, "")
	if err != nil {
		// Fallback to Raft commits if blockchain query fails
		raftCommits, raftErr := s.getRecentCommits(10000)
		if raftErr != nil {
			return nil, fmt.Errorf("failed to get commits from blockchain and Raft: blockchain=%v, raft=%v", err, raftErr)
		}
		stats.TotalCommits = len(raftCommits)
	} else {
		// Get history to find actual block heights
		history, err := client.GetIdentityHistory(identityName, 0, 0)
		if err != nil {
			// Use all commits if history fails (will overcount)
			stats.TotalCommits = len(commits)
		} else {
			// Build history map to get actual block heights
			historyMap := make(map[string]int64)
			const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"
			if history != nil && len(history.History) > 0 {
				for i := len(history.History) - 1; i >= 0; i-- {
					entry := history.History[i]
					if entry.Identity.ContentMultiMap == nil {
						continue
					}
					for keyID, entries := range entry.Identity.ContentMultiMap {
						if entryList, ok := entries.([]interface{}); ok {
							for _, item := range entryList {
								if entryMap, ok := item.(map[string]interface{}); ok {
									if lmsIndex, ok := entryMap[mapKey].(string); ok {
										mapKeyStr := fmt.Sprintf("%s:%s", keyID, lmsIndex)
										if existingHeight, exists := historyMap[mapKeyStr]; !exists {
											historyMap[mapKeyStr] = entry.Height
										} else if entry.Height < existingHeight {
											historyMap[mapKeyStr] = entry.Height
										}
									}
								}
							}
						}
					}
				}
			}

			// Count commits that are above bootstrap height
			blockchainCommitCount := 0
			for _, commit := range commits {
				mapKeyStr := fmt.Sprintf("%s:%s", commit.KeyID, commit.LMSIndex)
				blockHeight := commit.BlockHeight
				if histHeight, exists := historyMap[mapKeyStr]; exists {
					blockHeight = histHeight
				}
				if bootstrapHeight <= 0 || blockHeight >= bootstrapHeight {
					blockchainCommitCount++
				}
			}
			stats.TotalCommits = blockchainCommitCount
		}
	}

	// Get Raft commits for other stats (key count, chain validation)
	raftCommits, err := s.getRecentCommits(10000)
	if err != nil {
		// Continue with blockchain stats if Raft fails
		raftCommits = []CommitInfo{}
	}

	// Count unique keys from Raft commits
	keySet := make(map[string]bool)
	for _, commit := range raftCommits {
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

	if len(raftCommits) > 0 {
		stats.LastCommit = raftCommits[0].Timestamp
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
