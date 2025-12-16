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

	// Get all commits
	commits, err := client.QueryAttestationCommits(identityName, "")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query blockchain: %v", err), http.StatusInternalServerError)
		return
	}

	// Enrich commits with key_id labels from Raft
	enrichedCommits := make([]map[string]interface{}, len(commits))
	for i, commit := range commits {
		enrichedCommit := map[string]interface{}{
			"key_id":       commit.KeyID,
			"pubkey_hash":  commit.PubkeyHash,
			"lms_index":    commit.LMSIndex,
			"block_height": commit.BlockHeight,
			"txid":         commit.TxID,
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
