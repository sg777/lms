package explorer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/verifiable-state-chains/lms/blockchain"
)

// handleBlockchain returns all blockchain commits from Verus identity
func (s *ExplorerServer) handleBlockchain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For now, use default Verus client configuration
	// In production, this should come from config
	client := blockchain.NewVerusClient(
		"http://127.0.0.1:22778",
		"user1172159772",
		"pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da",
	)

	identityName := "sg777z.chips.vrsc@"

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
			"block_height":  commit.BlockHeight,
			"txid":         commit.TxID,
			"timestamp":    commit.Timestamp,
			"key_id_label": "", // Will be populated from Raft
		}

		// Try to get key_id label from Raft by querying chain endpoint
		// We'll try to query using the normalized key ID (KeyID) as if it were a pubkey_hash
		// If that doesn't work, we'll try querying all chains and matching
		keyIDLabel := s.lookupKeyIDLabelFromRaft(commit.KeyID)
		if keyIDLabel != "" {
			enrichedCommit["key_id_label"] = keyIDLabel
		}

		enrichedCommits[i] = enrichedCommit
	}

	// Get blockchain info
	height, err := client.GetBlockHeight()
	if err != nil {
		height = 0
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":        true,
		"identity":       identityName,
		"block_height":   height,
		"commit_count":   len(commits),
		"commits":        enrichedCommits,
	})
}

// lookupKeyIDLabelFromRaft tries to find the key_id label for a normalized key ID by querying Raft
func (s *ExplorerServer) lookupKeyIDLabelFromRaft(normalizedKeyID string) string {
	// Try querying Raft chain endpoint for each pubkey_hash we know about
	// Since we don't have a direct mapping, we'll query all known keys from Raft
	// and match by checking if their normalized ID matches
	
	// Query all keys from Raft
	for _, endpoint := range s.raftEndpoints {
		// Try to get chain by normalized key ID (might work if Raft stores it)
		url := fmt.Sprintf("%s/pubkey_hash/%s/chain", endpoint, normalizedKeyID)
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
						if keyID, ok := firstEntry["key_id"].(string); ok && keyID != "" {
							return keyID
						}
					}
				}
			}
		}

		// Also try querying by key_id (in case the normalized ID is stored as key_id)
		url = fmt.Sprintf("%s/chain/%s", endpoint, normalizedKeyID)
		resp, err = http.Get(url)
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
						if keyID, ok := firstEntry["key_id"].(string); ok && keyID != "" {
							return keyID
						}
					}
				}
			}
		}
	}

	return ""
}

