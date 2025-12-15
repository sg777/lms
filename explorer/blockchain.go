package explorer

import (
	"encoding/json"
	"fmt"
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
		"commits":        commits,
	})
}

