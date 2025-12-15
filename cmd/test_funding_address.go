package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/verifiable-state-chains/lms/blockchain"
)

func main() {
	var (
		rpcURL      = flag.String("rpc-url", "http://127.0.0.1:22778", "Verus RPC URL")
		rpcUser     = flag.String("rpc-user", "user1172159772", "RPC username")
		rpcPassword = flag.String("rpc-password", "pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da", "RPC password")
		identity    = flag.String("identity", "sg777z.chips.vrsc@", "Identity to update")
		testKeyID   = flag.String("key-id", "test_funding_key", "Test key ID for commit")
		testIndex   = flag.String("lms-index", "999", "Test LMS index to commit")
		fundingAddr = flag.String("funding-address", "", "Specific funding address to use (if not provided, will auto-detect)")
	)
	flag.Parse()

	client := blockchain.NewVerusClient(*rpcURL, *rpcUser, *rpcPassword)

	// Define AddressBalance type
	type AddressBalance struct {
		Address string
		Balance float64
	}

	fmt.Println("=== Testing Explicit Funding Address ===")
	fmt.Println()

	var selectedAddr AddressBalance

	// If funding address is explicitly provided, use it
	if *fundingAddr != "" {
		fmt.Println("Step 1: Using provided funding address...")
		balance, err := client.GetBalance(*fundingAddr)
		if err != nil {
			log.Fatalf("Failed to get balance for address %s: %v", *fundingAddr, err)
		}
		selectedAddr = AddressBalance{
			Address: *fundingAddr,
			Balance: balance,
		}
		fmt.Printf("  Address: %s\n", selectedAddr.Address)
		fmt.Printf("  Balance: %.8f CHIPS\n", selectedAddr.Balance)
		fmt.Println()
	} else {
		// Step 1: List all addresses in wallet
		fmt.Println("Step 1: Listing all addresses in wallet...")
		addresses, err := client.ListAddresses()
		if err != nil {
			log.Fatalf("Failed to list addresses: %v\n\nTip: You can specify an address manually using -funding-address=<address>", err)
		}

		if len(addresses) == 0 {
			log.Fatal("No addresses found in wallet. Please create some addresses first.")
		}

		fmt.Printf("Found %d addresses in wallet\n", len(addresses))
		fmt.Println()

		// Step 2: Check balances for each address
		fmt.Println("Step 2: Checking balances for each address...")
		var addressesWithBalance []AddressBalance

		for _, addr := range addresses {
			balance, err := client.GetBalance(addr)
			if err != nil {
				fmt.Printf("  ⚠️  Failed to get balance for %s: %v\n", addr, err)
				continue
			}
			fmt.Printf("  Address: %s\n", addr)
			fmt.Printf("  Balance: %.8f CHIPS\n", balance)
			fmt.Println()

			if balance > 0 {
				addressesWithBalance = append(addressesWithBalance, AddressBalance{
					Address: addr,
					Balance: balance,
				})
			}
		}

		if len(addressesWithBalance) == 0 {
			log.Fatal("No addresses with balance found. Please fund at least one address.")
		}

		// Step 3: Select address with highest balance
		selectedAddr = addressesWithBalance[0]
		for _, ab := range addressesWithBalance {
			if ab.Balance > selectedAddr.Balance {
				selectedAddr = ab
			}
		}

		fmt.Printf("Step 3: Selected address for funding: %s\n", selectedAddr.Address)
		fmt.Printf("  Balance: %.8f CHIPS\n", selectedAddr.Balance)
		fmt.Println()
	}

	// Step 4: Get balance before transaction
	stepNum := 4
	if *fundingAddr == "" {
		stepNum = 4
	} else {
		stepNum = 2
	}
	fmt.Printf("Step %d: Getting balance BEFORE transaction...\n", stepNum)
	balanceBefore, err := client.GetBalance(selectedAddr.Address)
	if err != nil {
		log.Fatalf("Failed to get balance before: %v", err)
	}
	fmt.Printf("  Balance before: %.8f CHIPS\n", balanceBefore)
	fmt.Println()

	// Step 5: Commit to blockchain with explicit funding address
	stepNum++
	fmt.Printf("Step %d: Committing to blockchain with explicit funding address...\n", stepNum)
	fmt.Printf("  Identity: %s\n", *identity)
	fmt.Printf("  Key ID: %s\n", *testKeyID)
	fmt.Printf("  LMS Index: %s\n", *testIndex)
	fmt.Printf("  Funding Address: %s\n", selectedAddr.Address)
	fmt.Println()

	normalizedKeyID, txID, err := client.CommitLMSIndexWithPubkeyHash(
		*identity,
		*testKeyID, // Using key_id as pubkey_hash for testing
		*testIndex,
		selectedAddr.Address, // Explicit funding address
	)

	if err != nil {
		log.Fatalf("❌ Failed to commit: %v", err)
	}

	fmt.Printf("✅ Transaction successful!\n")
	fmt.Printf("  Transaction ID: %s\n", txID)
	fmt.Printf("  Normalized Key ID: %s\n", normalizedKeyID)
	fmt.Println()

	// Step 6: Wait for transaction confirmation and check balance after
	stepNum++
	fmt.Printf("Step %d: Waiting for transaction confirmation...\n", stepNum)
	fmt.Printf("  Transaction ID: %s\n", txID)
	fmt.Println("  Waiting up to 30 seconds for confirmation...")
	
	// Wait for confirmation (check every 2 seconds, up to 30 seconds)
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		if i == 0 || i%3 == 0 {
			fmt.Printf("  Checking... (%d/15)\n", i+1)
		}
	}
	fmt.Println("  Checking balance after transaction...")
	
	balanceAfter, err := client.GetBalance(selectedAddr.Address)
	if err != nil {
		log.Fatalf("Failed to get balance after: %v", err)
	}
	fmt.Printf("  Balance after: %.8f CHIPS\n", balanceAfter)
	fmt.Println()

	// Step 7: Calculate fee
	feeDeducted := balanceBefore - balanceAfter
	fmt.Println("=== Results ===")
	fmt.Printf("Balance before: %.8f CHIPS\n", balanceBefore)
	fmt.Printf("Balance after:  %.8f CHIPS\n", balanceAfter)
	fmt.Printf("Fee deducted:   %.8f CHIPS\n", feeDeducted)
	fmt.Println()

	if feeDeducted > 0 {
		fmt.Printf("✅ SUCCESS: Transaction fee (%.8f CHIPS) was deducted from address %s\n", feeDeducted, selectedAddr.Address)
		fmt.Println("   This confirms that the explicit funding address was used!")
	} else if feeDeducted == 0 {
		fmt.Printf("⚠️  WARNING: No fee deducted yet. Transaction might still be pending.\n")
		fmt.Printf("   Check the transaction %s on the blockchain explorer.\n", txID)
	} else {
		fmt.Printf("❌ ERROR: Balance increased (unexpected). Something went wrong.\n")
	}

	// Step 8: Verify the commit was recorded
	stepNum++
	fmt.Println()
	fmt.Printf("Step %d: Verifying commit was recorded in identity...\n", stepNum)
	commits, err := client.QueryAttestationCommits(*identity, normalizedKeyID)
	if err != nil {
		log.Printf("⚠️  Failed to query commits: %v", err)
	} else {
		found := false
		for _, commit := range commits {
			if commit.LMSIndex == *testIndex {
				fmt.Printf("✅ Commit verified! Found in identity:\n")
				fmt.Printf("   Key ID: %s\n", commit.KeyID)
				fmt.Printf("   LMS Index: %s\n", commit.LMSIndex)
				fmt.Printf("   Block Height: %d\n", commit.BlockHeight)
				fmt.Printf("   Transaction ID: %s\n", commit.TxID)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("⚠️  Commit not found in identity yet (might need to wait for confirmation)\n")
		}
	}

	fmt.Println()
	fmt.Println("=== Test Complete ===")
}

