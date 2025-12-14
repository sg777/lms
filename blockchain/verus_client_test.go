package blockchain

import (
	"fmt"
	"testing"
)

// TestVerusClientBasic tests basic RPC connectivity
// This is a manual test - uncomment and run to test with real CHIPS chain
func TestVerusClientBasic(t *testing.T) {
	t.Skip("Skipping manual test - uncomment to test with real CHIPS chain")

	// RPC credentials from config file
	rpcURL := "http://127.0.0.1:22778"
	rpcUser := "user1172159772"
	rpcPassword := "pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da"

	client := NewVerusClient(rpcURL, rpcUser, rpcPassword)

	// Test getblockchaininfo
	info, err := client.GetBlockchainInfo()
	if err != nil {
		t.Fatalf("GetBlockchainInfo failed: %v", err)
	}
	fmt.Printf("Blockchain Info: %+v\n", info)

	// Test getblockheight
	height, err := client.GetBlockHeight()
	if err != nil {
		t.Fatalf("GetBlockHeight failed: %v", err)
	}
	fmt.Printf("Current block height: %d\n", height)

	// Test getbestblockhash
	hash, err := client.GetBestBlockHash()
	if err != nil {
		t.Fatalf("GetBestBlockHash failed: %v", err)
	}
	fmt.Printf("Best block hash: %s\n", hash)
}

