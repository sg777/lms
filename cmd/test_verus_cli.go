package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/verifiable-state-chains/lms/blockchain"
)

func main() {
	var (
		rpcURL      = flag.String("rpc-url", "http://127.0.0.1:22778", "Verus RPC URL")
		rpcUser     = flag.String("rpc-user", "user1172159772", "RPC username")
		rpcPassword = flag.String("rpc-password", "pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da", "RPC password")
		identity    = flag.String("identity", "sg777z.chips.vrsc@", "Identity to use")
		action      = flag.String("action", "info", "Action: info, commit, query, history")
		keyID       = flag.String("key-id", "", "LMS key ID (required for commit/query/history)")
		lmsIndex    = flag.String("lms-index", "", "LMS index to commit (required for commit)")
		heightStart = flag.Int64("height-start", 0, "Starting block height for history (default: 0)")
		heightEnd   = flag.Int64("height-end", 0, "Ending block height for history (default: 0 = current)")
	)
	flag.Parse()

	client := blockchain.NewVerusClient(*rpcURL, *rpcUser, *rpcPassword)

	switch *action {
	case "info":
		handleInfo(client, *identity)
	case "commit":
		if *keyID == "" || *lmsIndex == "" {
			fmt.Fprintf(os.Stderr, "Error: key-id and lms-index are required for commit action\n")
			flag.Usage()
			os.Exit(1)
		}
		handleCommit(client, *identity, *keyID, *lmsIndex)
	case "query":
		handleQuery(client, *identity, *keyID)
	case "history":
		if *keyID == "" {
			fmt.Fprintf(os.Stderr, "Error: key-id is required for history action\n")
			flag.Usage()
			os.Exit(1)
		}
		handleHistory(client, *identity, *keyID, *heightStart, *heightEnd)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown action '%s'. Use: info, commit, or query\n", *action)
		flag.Usage()
		os.Exit(1)
	}
}

func handleInfo(client *blockchain.VerusClient, identityName string) {
	fmt.Printf("=== Blockchain Info ===\n")
	height, err := client.GetBlockHeight()
	if err != nil {
		log.Fatalf("Failed to get block height: %v", err)
	}
	fmt.Printf("Current block height: %d\n\n", height)

	fmt.Printf("=== Identity Info: %s ===\n", identityName)
	identity, err := client.GetIdentity(identityName)
	if err != nil {
		log.Fatalf("Failed to get identity: %v", err)
	}

	fmt.Printf("Friendly Name: %s\n", identity.FriendlyName)
	fmt.Printf("Fully Qualified: %s\n", identity.FullyQualifiedName)
	fmt.Printf("Status: %s\n", identity.Status)
	fmt.Printf("Block Height: %d\n", identity.BlockHeight)
	fmt.Printf("TXID: %s\n", identity.TxID)

	if identity.Identity.ContentMultiMap != nil && len(identity.Identity.ContentMultiMap) > 0 {
		fmt.Printf("\n=== Content MultiMap ===\n")
		jsonData, _ := json.MarshalIndent(identity.Identity.ContentMultiMap, "", "  ")
		fmt.Printf("%s\n", jsonData)
	} else {
		fmt.Printf("\nContent MultiMap: (empty)\n")
	}
}

func handleCommit(client *blockchain.VerusClient, identityName, keyID, lmsIndex string) {
	fmt.Printf("=== Committing LMS Index to Identity ===\n")
	fmt.Printf("Identity: %s\n", identityName)
	fmt.Printf("Key ID: %s\n", keyID)
	fmt.Printf("LMS Index: %s\n\n", lmsIndex)

	txID, err := client.UpdateIdentity(identityName, keyID, lmsIndex)
	if err != nil {
		log.Fatalf("Failed to commit: %v", err)
	}

	fmt.Printf("✅ Success! Transaction ID: %s\n", txID)
	fmt.Printf("\nNote: The identity update may take a few moments to confirm.\n")
	fmt.Printf("Use 'query' action to verify the commit.\n")
}

func handleQuery(client *blockchain.VerusClient, identityName, keyID string) {
	fmt.Printf("=== Querying LMS Index Commits ===\n")
	fmt.Printf("Identity: %s\n", identityName)
	if keyID != "" {
		fmt.Printf("Key ID: %s\n", keyID)
	} else {
		fmt.Printf("Key ID: (all)\n")
	}
	fmt.Println()

	commits, err := client.QueryAttestationCommits(identityName, keyID)
	if err != nil {
		log.Fatalf("Failed to query commits: %v", err)
	}

	if len(commits) == 0 {
		fmt.Printf("No commits found.\n")
		return
	}

	fmt.Printf("Found %d commit(s):\n\n", len(commits))
	for i, commit := range commits {
		fmt.Printf("Commit %d:\n", i+1)
		fmt.Printf("  Key ID: %s\n", commit.KeyID)
		fmt.Printf("  LMS Index: %s\n", commit.LMSIndex)
		fmt.Printf("  Block Height: %d\n", commit.BlockHeight)
		fmt.Printf("  TXID: %s\n", commit.TxID)
		fmt.Println()
	}
}

func handleHistory(client *blockchain.VerusClient, identityName, keyID string, heightStart, heightEnd int64) {
	fmt.Printf("=== LMS Index History for Key ID ===\n")
	fmt.Printf("Identity: %s\n", identityName)
	fmt.Printf("Key ID: %s\n", keyID)
	if heightStart > 0 || heightEnd > 0 {
		fmt.Printf("Height Range: %d to ", heightStart)
		if heightEnd == 0 {
			fmt.Printf("current\n")
		} else {
			fmt.Printf("%d\n", heightEnd)
		}
	}
	fmt.Println()

	commits, err := client.GetLMSIndexHistory(identityName, keyID, heightStart, heightEnd)
	if err != nil {
		log.Fatalf("Failed to get history: %v", err)
	}

	if len(commits) == 0 {
		fmt.Printf("No history found.\n")
		return
	}

	fmt.Printf("Found %d historical commit(s) (ordered by block height):\n\n", len(commits))
	for i, commit := range commits {
		fmt.Printf("Entry %d:\n", i+1)
		fmt.Printf("  LMS Index: %s\n", commit.LMSIndex)
		fmt.Printf("  Block Height: %d\n", commit.BlockHeight)
		fmt.Printf("  TXID: %s\n", commit.TxID)
		fmt.Println()
	}
}

