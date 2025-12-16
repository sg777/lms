package explorer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
)

// handleBlockchain returns all blockchain commits from Verus identity
func (s *ExplorerServer) handleBlockchain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For now, use env-configured Verus client configuration
	client := newVerusClientFromEnv()
	identityName := verusIdentityName()

	// Get all commits from current identity state
	commits, err := client.QueryAttestationCommits(identityName, "")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query blockchain: %v", err), http.StatusInternalServerError)
		return
	}

	// Get identity history to find actual block heights for each commit
	// This is needed because QueryAttestationCommits only returns current identity block height
	history, err := client.GetIdentityHistory(identityName, 0, 0)
	if err != nil {
		// If history fails, log the error but continue with current state
		log.Printf("[WARNING] Failed to get identity history: %v. Using current block height for all commits.", err)
		history = nil
	}

	// Build a map of (keyID, lmsIndex) -> (blockHeight, txID) from history
	// We need to find when each lms_index was first committed for each keyID
	// Process history from OLDEST to NEWEST to capture the first time each lms_index appears
	historyMap := make(map[string]int64) // key: "keyID:lmsIndex", value: blockHeight (first commit)
	txidMap := make(map[string]string)   // key: "keyID:lmsIndex", value: txID (first commit)
	if history != nil && len(history.History) > 0 {
		const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"
		log.Printf("[INFO] Processing %d history entries to find commit block heights", len(history.History))
		
		// Process ALL history entries and find the OLDEST (lowest) block height for each commit
		// We want the FIRST time each lms_index was committed, which is the oldest block height
		// Process in both directions to ensure we catch the oldest regardless of history order
		for i := 0; i < len(history.History); i++ {
			entry := history.History[i]
			if entry.Identity.ContentMultiMap == nil {
				continue
			}
			// For each key_id in this historical entry
			for keyID, entries := range entry.Identity.ContentMultiMap {
				if entryList, ok := entries.([]interface{}); ok {
					for _, item := range entryList {
						if entryMap, ok := item.(map[string]interface{}); ok {
							if lmsIndex, ok := entryMap[mapKey].(string); ok {
								// Create map key: "keyID:lmsIndex"
								mapKeyStr := fmt.Sprintf("%s:%s", keyID, lmsIndex)
								// Store the OLDEST (lowest) block height for this commit
								// If we haven't seen it before, store it
								// If we have seen it, only update if this entry is older (lower height)
								if existingHeight, exists := historyMap[mapKeyStr]; !exists {
									// First time seeing this commit - store it
									historyMap[mapKeyStr] = entry.Height
									txidMap[mapKeyStr] = entry.Output.TxID
									log.Printf("[DEBUG] Found commit: keyID=%s, lmsIndex=%s, blockHeight=%d, txID=%s", keyID, lmsIndex, entry.Height, entry.Output.TxID)
								} else if entry.Height < existingHeight {
									// Found an older entry (lower block height) - update to the oldest
									historyMap[mapKeyStr] = entry.Height
									txidMap[mapKeyStr] = entry.Output.TxID
									log.Printf("[DEBUG] Updated commit to older block: keyID=%s, lmsIndex=%s, oldHeight=%d, newHeight=%d, txID=%s", keyID, lmsIndex, existingHeight, entry.Height, entry.Output.TxID)
								}
							}
						}
					}
				}
			}
		}
		log.Printf("[INFO] Built history map with %d entries", len(historyMap))
	} else if history == nil {
		log.Printf("[WARNING] Identity history is nil - cannot determine actual commit block heights")
	} else {
		log.Printf("[WARNING] Identity history is empty - cannot determine actual commit block heights")
	}

	// Enrich commits with key_id labels from Raft and actual block heights from history
	enrichedCommits := make([]map[string]interface{}, len(commits))
	matchedCount := 0
	for i, commit := range commits {
		// Try to get actual block height from history
		blockHeight := commit.BlockHeight
		txid := commit.TxID
		mapKeyStr := fmt.Sprintf("%s:%s", commit.KeyID, commit.LMSIndex)
		if histHeight, exists := historyMap[mapKeyStr]; exists {
			blockHeight = histHeight
			matchedCount++
			if histTxid, exists := txidMap[mapKeyStr]; exists {
				txid = histTxid
			}
			log.Printf("[DEBUG] Matched commit: keyID=%s, lmsIndex=%s, using blockHeight=%d (was %d)", commit.KeyID, commit.LMSIndex, blockHeight, commit.BlockHeight)
		} else {
			log.Printf("[DEBUG] No history match for: keyID=%s, lmsIndex=%s, using current blockHeight=%d", commit.KeyID, commit.LMSIndex, commit.BlockHeight)
		}

		enrichedCommit := map[string]interface{}{
			"key_id":       commit.KeyID,
			"pubkey_hash":  commit.PubkeyHash,
			"lms_index":    commit.LMSIndex,
			"block_height": blockHeight,
			"txid":         txid,
			"timestamp":    commit.Timestamp,
			"key_id_label": "", // Will be populated from Raft
		}

		// Try to get key_id label from Raft
		// commit.KeyID is the normalized VDXF ID from Verus
		// We need to match it by querying all keys from Raft and computing their normalized IDs
		keyIDLabel := s.lookupKeyIDLabelFromRaft(commit.KeyID)
		if keyIDLabel != "" {
			enrichedCommit["key_id_label"] = keyIDLabel
		}

		enrichedCommits[i] = enrichedCommit
	}
	
	log.Printf("[INFO] Matched %d out of %d commits with historical block heights", matchedCount, len(commits))

	// Sort commits by block height (descending - highest/newest first)
	// Then by key_id and lms_index for consistent ordering
	sort.Slice(enrichedCommits, func(i, j int) bool {
		heightI, _ := enrichedCommits[i]["block_height"].(int64)
		heightJ, _ := enrichedCommits[j]["block_height"].(int64)
		keyIDI, _ := enrichedCommits[i]["key_id"].(string)
		keyIDJ, _ := enrichedCommits[j]["key_id"].(string)
		lmsIndexI, _ := enrichedCommits[i]["lms_index"].(string)
		lmsIndexJ, _ := enrichedCommits[j]["lms_index"].(string)

		// Primary sort: block height (descending - highest first)
		if heightJ != heightI {
			return heightJ < heightI // Descending order
		}

		// Secondary sort: key_id (ascending)
		if keyIDJ != keyIDI {
			return keyIDI < keyIDJ // Ascending order
		}

		// Tertiary sort: lms_index (descending - highest first)
		return lmsIndexJ < lmsIndexI // Descending order
	})

	// Get blockchain info
	height, err := client.GetBlockHeight()
	if err != nil {
		height = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"identity":     identityName,
		"block_height": height,
		"commit_count": len(commits),
		"commits":      enrichedCommits,
	})
}

// lookupKeyIDLabelFromRaft tries to find the key_id label for a normalized VDXF ID by querying Raft
// Since Verus normalizes keys, we need to query all keys from Raft and match by computing their normalized IDs
func (s *ExplorerServer) lookupKeyIDLabelFromRaft(normalizedKeyID string) string {
	// First, try querying directly with the normalized ID (in case Raft stores it)
	for _, endpoint := range s.raftEndpoints {
		// Try querying by normalized ID as if it were a pubkey_hash
		url := fmt.Sprintf("%s/pubkey_hash/%s/chain", endpoint, normalizedKeyID)
		resp, err := http.Get(url)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var chainResp map[string]interface{}
				if err := json.Unmarshal(body, &chainResp); err == nil {
					if chain, ok := chainResp["chain"].([]interface{}); ok && len(chain) > 0 {
						if firstEntry, ok := chain[0].(map[string]interface{}); ok {
							if keyID, ok := firstEntry["key_id"].(string); ok && keyID != "" {
								return keyID
							}
						}
					}
				}
			}
		}
	}

	// If direct query failed, get all keys from Raft and match by computing normalized IDs
	allKeys, err := s.getAllKeys()
	if err != nil || len(allKeys) == 0 {
		return ""
	}

	// For each key, query its chain by pubkey_hash to match normalized ID
	// We need to get the pubkey_hash for each key first
	// Try querying by key_id to get the chain, then extract pubkey_hash
	for _, keyID := range allKeys {
		// First, try to get pubkey_hash by querying the key's chain
		// The chain endpoint might have pubkey_hash, or we can query by pubkey_hash directly
		// But we don't have the pubkey_hash yet, so let's try a different approach:
		// Query the /pubkey_hash endpoint for all possible hashes? No, that's not feasible.

		// Better approach: Query chain by key_id, get public_key, compute pubkey_hash
		for _, endpoint := range s.raftEndpoints {
			// Try querying chain by key_id to get public_key
			url := fmt.Sprintf("%s/key/%s/chain", endpoint, keyID)
			resp, err := http.Get(url)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				var chainResp map[string]interface{}
				if err := json.Unmarshal(body, &chainResp); err == nil {
					if chain, ok := chainResp["chain"].([]interface{}); ok && len(chain) > 0 {
						if firstEntry, ok := chain[0].(map[string]interface{}); ok {
							// Try to get pubkey_hash directly from entry
							var pubkeyHash string
							if ph, ok := firstEntry["pubkey_hash"].(string); ok && ph != "" {
								pubkeyHash = ph
							} else if publicKey, ok := firstEntry["public_key"].(string); ok && publicKey != "" {
								// Compute pubkey_hash from public_key (SHA-256)
								hash := sha256.Sum256([]byte(publicKey))
								pubkeyHash = fmt.Sprintf("%x", hash)
							}

							if pubkeyHash != "" {
								// Compute normalized VDXF ID for this pubkey_hash
								client := newVerusClientFromEnv()
								computedNormalized, err := client.GetVDXFID(pubkeyHash)
								if err == nil && computedNormalized == normalizedKeyID {
									return keyID
								}
							}
						}
					}
				}
			}
		}
	}

	return ""
}
