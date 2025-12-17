package explorer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"
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

	// Get bootstrap block height from environment (if set)
	// Commits before this block height will be filtered out
	bootstrapHeight := getBootstrapBlockHeight()
	if bootstrapHeight > 0 {
		log.Printf("[INFO] Bootstrap block height configured: %d - filtering commits before this height", bootstrapHeight)
	}

	// Get identity history to retrieve ALL commits (including delete records)
	// QueryAttestationCommits only returns the CURRENT state (latest index per key),
	// but we need ALL commits including delete records, so we must use history
	history, err := client.GetIdentityHistory(identityName, 0, 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get identity history: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("[INFO] Retrieved identity history with %d entries", len(history.History))

	// Extract ALL commits from history (each history entry represents one commit)
	// This includes create, sign, sync, and delete records
	const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"
	type historyCommit struct {
		KeyID       string
		LMSIndex    string
		BlockHeight int64
		TxID        string
	}

	historyCommits := make([]historyCommit, 0)
	firstSeen := make(map[string]bool) // key: "keyID:lmsIndex" to track if we've seen this commit before

	// Process history entries from OLDEST to NEWEST
	// The first time we see a (keyID, lmsIndex) is when it was actually committed
	for _, entry := range history.History {
		if entry.Identity.ContentMultiMap == nil {
			continue
		}
		// For each key_id in this historical entry
		for keyID, entries := range entry.Identity.ContentMultiMap {
			if entryList, ok := entries.([]interface{}); ok {
				for _, item := range entryList {
					if entryMap, ok := item.(map[string]interface{}); ok {
						if lmsIndex, ok := entryMap[mapKey].(string); ok {
							// Create unique key: "keyID:lmsIndex" (without block height)
							commitKey := fmt.Sprintf("%s:%s", keyID, lmsIndex)
							// Only record the FIRST time we see this commit (actual commit block height)
							if !firstSeen[commitKey] {
								firstSeen[commitKey] = true
								historyCommits = append(historyCommits, historyCommit{
									KeyID:       keyID,
									LMSIndex:    lmsIndex,
									BlockHeight: entry.Height,
									TxID:        entry.Output.TxID,
								})
								log.Printf("[DEBUG] Found commit: keyID=%s, lmsIndex=%s, blockHeight=%d, txID=%s", keyID, lmsIndex, entry.Height, entry.Output.TxID)
							}
						}
					}
				}
			}
		}
	}

	log.Printf("[INFO] Extracted %d unique commits from history", len(historyCommits))

	// Cache key_id label lookups to avoid redundant queries
	keyIDLabelCache := make(map[string]string) // normalizedKeyID -> key_id_label

	// Filter commits by bootstrap block height and enrich with key_id labels
	filteredCount := 0

	enrichedCommits := make([]map[string]interface{}, 0, len(historyCommits))
	for _, commit := range historyCommits {
		// Filter by bootstrap block height if configured
		if bootstrapHeight > 0 && commit.BlockHeight < bootstrapHeight {
			filteredCount++
			log.Printf("[DEBUG] Filtering commit: keyID=%s, lmsIndex=%s, blockHeight=%d (below bootstrap %d)", commit.KeyID, commit.LMSIndex, commit.BlockHeight, bootstrapHeight)
			continue // Skip this commit
		}

		// Get key_id label from cache or lookup
		keyIDLabel := ""
		if cached, exists := keyIDLabelCache[commit.KeyID]; exists {
			keyIDLabel = cached
		} else {
			// Lookup key_id label from Raft by matching normalized VDXF ID
			keyIDLabel = s.lookupKeyIDLabelFromRaft(commit.KeyID)
			keyIDLabelCache[commit.KeyID] = keyIDLabel // Cache result (even if empty)
		}

		enrichedCommit := map[string]interface{}{
			"key_id":       commit.KeyID, // Canonical key ID (normalized VDXF ID)
			"pubkey_hash":  "",           // Not available from history alone
			"lms_index":    commit.LMSIndex,
			"block_height": commit.BlockHeight,
			"txid":         commit.TxID,
			"timestamp":    time.Time{}, // Not available from history
			"key_id_label": keyIDLabel,  // User-friendly key_id label from Raft
		}

		enrichedCommits = append(enrichedCommits, enrichedCommit)
	}

	log.Printf("[INFO] Total commits from history: %d, filtered out: %d, displaying: %d", len(historyCommits), filteredCount, len(enrichedCommits))

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
		"commit_count": len(enrichedCommits), // Use enrichedCommits count (after bootstrap filtering)
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
							var pubkeyHashHex string
							if ph, ok := firstEntry["pubkey_hash"].(string); ok && ph != "" {
								// pubkey_hash from Raft is in base64 format, convert to hex
								pubkeyHashBytes, err := base64.StdEncoding.DecodeString(ph)
								if err == nil {
									pubkeyHashHex = fmt.Sprintf("%x", pubkeyHashBytes)
								} else {
									// If decoding fails, try using as-is (might already be hex)
									pubkeyHashHex = ph
								}
							} else if publicKey, ok := firstEntry["public_key"].(string); ok && publicKey != "" {
								// Compute pubkey_hash from public_key
								// public_key is base64 encoded EC public key, decode it first
								var pubKeyBytes []byte
								if decoded, err := base64.StdEncoding.DecodeString(publicKey); err == nil {
									pubKeyBytes = decoded
								} else {
									pubKeyBytes = []byte(publicKey)
								}
								hash := sha256.Sum256(pubKeyBytes)
								pubkeyHashHex = fmt.Sprintf("%x", hash)
							}

							if pubkeyHashHex != "" {
								// Compute normalized VDXF ID for this pubkey_hash (hex format)
								client := newVerusClientFromEnv()
								computedNormalized, err := client.GetVDXFID(pubkeyHashHex)
								if err == nil && computedNormalized == normalizedKeyID {
									log.Printf("[DEBUG] Matched normalized ID %s to key_id label: %s", normalizedKeyID, keyID)
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
