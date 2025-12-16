package explorer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
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
		// If history fails, fall back to current state (with current block height)
		history = nil
	}

	// Build a map of (keyID, lmsIndex) -> (blockHeight, txID) from history
	// We need to find when each lms_index was first committed for each keyID
	historyMap := make(map[string]int64) // key: "keyID:lmsIndex", value: blockHeight (first commit)
	txidMap := make(map[string]string)   // key: "keyID:lmsIndex", value: txID (first commit)
	if history != nil {
		const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"
		// Process history in reverse chronological order to find the first commit of each lms_index
		// History is typically ordered newest to oldest, but we'll process from oldest to newest
		// to catch the first time each lms_index appears
		for i := len(history.History) - 1; i >= 0; i-- {
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
								// Store the block height if not already stored (first time we see this lms_index)
								// We process from oldest to newest, so the first entry we find is the commit block
								if _, exists := historyMap[mapKeyStr]; !exists {
									historyMap[mapKeyStr] = entry.Height
									txidMap[mapKeyStr] = entry.Output.TxID
								}
							}
						}
					}
				}
			}
		}
	}

	// Enrich commits with key_id labels from Raft and actual block heights from history
	enrichedCommits := make([]map[string]interface{}, len(commits))
	for i, commit := range commits {
		// Try to get actual block height from history
		blockHeight := commit.BlockHeight
		txid := commit.TxID
		mapKeyStr := fmt.Sprintf("%s:%s", commit.KeyID, commit.LMSIndex)
		if histHeight, exists := historyMap[mapKeyStr]; exists {
			blockHeight = histHeight
			if histTxid, exists := txidMap[mapKeyStr]; exists {
				txid = histTxid
			}
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
