package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/verifiable-state-chains/lms/hsm_client"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	// Parse flags after command
	flagSet := flag.NewFlagSet(command, flag.ExitOnError)
	serverURL := flagSet.String("server", "http://159.69.23.29:9090", "HSM server URL")
	keyID := flagSet.String("key-id", "", "Key ID (for generate/sign command)")
	message := flagSet.String("msg", "", "Message to sign (for sign command)")
	raftEndpoint := flagSet.String("raft", "http://159.69.23.29:8080", "Raft cluster endpoint (for query command)")

	flagSet.Parse(os.Args[2:])

	client := hsm_client.NewHSMClient(*serverURL)

	switch command {
	case "help", "--help", "-h":
		printHelp()
		
	case "generate":
		key, err := client.GenerateKey(*keyID)
		if err != nil {
			log.Fatalf("Failed to generate key: %v", err)
		}
		fmt.Printf("✅ Generated LMS key:\n")
		fmt.Printf("   Key ID: %s\n", key.KeyID)
		fmt.Printf("   Index:  %d\n", key.Index)
		
	case "list":
		keys, err := client.ListKeys()
		if err != nil {
			log.Fatalf("Failed to list keys: %v", err)
		}
		
		if len(keys) == 0 {
			fmt.Println("No keys found.")
			return
		}
		
		fmt.Printf("📋 Available keys (%d):\n\n", len(keys))
		for i, key := range keys {
			fmt.Printf("  %d. Key ID: %s\n", i+1, key.KeyID)
			fmt.Printf("     Index:  %d\n", key.Index)
			if key.Created != "" {
				fmt.Printf("     Created: %s\n", key.Created)
			}
			fmt.Println()
		}
		
	case "sign":
		if *keyID == "" {
			log.Fatal("key-id is required for sign command")
		}
		if *message == "" {
			log.Fatal("msg is required for sign command")
		}
		
		result, err := client.Sign(*keyID, *message)
		if err != nil {
			log.Fatalf("Failed to sign: %v", err)
		}
		
		fmt.Printf("✅ Signed message:\n")
		fmt.Printf("   Key ID: %s\n", result.KeyID)
		fmt.Printf("   Index:  %d\n", result.Index)
		fmt.Printf("   Signature: %s\n", result.Signature)
		
	case "query":
		if *keyID == "" {
			log.Fatal("key-id is required for query command")
		}
		
		index, exists, err := hsm_client.QueryKeyIndex(*raftEndpoint, *keyID)
		if err != nil {
			log.Fatalf("Failed to query: %v", err)
		}
		
		if exists {
			fmt.Printf("Key ID: %s\n", *keyID)
			fmt.Printf("Last Index: %d\n", index)
		} else {
			fmt.Printf("Key ID: %s\n", *keyID)
			fmt.Printf("Status: Not found (no index committed yet)\n")
		}
		
	case "chain":
		if *keyID == "" {
			log.Fatal("key-id is required for chain command")
		}
		
		chain, err := hsm_client.GetKeyChain(*raftEndpoint, *keyID)
		if err != nil {
			log.Fatalf("Failed to get chain: %v", err)
		}
		
		if !chain.Exists || chain.Count == 0 {
			fmt.Printf("Key ID: %s\n", *keyID)
			fmt.Printf("Status: Not found (no entries in chain)\n")
			return
		}
		
		fmt.Printf("🔗 Hash Chain for Key ID: %s (%d entries)\n", chain.KeyID, chain.Count)
		
		// Show verification status
		if chain.Verification != nil {
			if chain.Verification.Valid {
				fmt.Printf("✅ Chain Integrity: VALID\n\n")
			} else {
				fmt.Printf("❌ Chain Integrity: BROKEN\n")
				fmt.Printf("   Error: %s\n", chain.Verification.Error)
				if chain.Verification.BreakIndex >= 0 {
					fmt.Printf("   Break at entry: %d\n", chain.Verification.BreakIndex+1)
				}
				fmt.Println()
			}
		}
		
		for i, entry := range chain.Chain {
			fmt.Printf("--- Entry %d (LMS Index: %d) ---\n", i+1, entry.Index)
			
			// Show previous hash with link status
			fmt.Printf("  Previous Hash: %s\n", entry.PreviousHash)
			if i == 0 && entry.PreviousHash == "0000000000000000000000000000000000000000000000000000000000000000" {
				fmt.Printf("                [GENESIS - First entry in chain] ✓\n")
			} else if i > 0 {
				prevEntry := chain.Chain[i-1]
				if entry.PreviousHash == prevEntry.Hash {
					fmt.Printf("                ✓ Links to Entry %d's hash\n", i)
				} else {
					fmt.Printf("                ✗ BROKEN: Does NOT match Entry %d's hash!\n", i)
					fmt.Printf("                  Expected: %s\n", prevEntry.Hash)
					fmt.Printf("                  Got:      %s\n", entry.PreviousHash)
				}
			}
			
			fmt.Printf("  Hash:          %s\n", entry.Hash)
			
			// Show if this will link to next entry
			if i < len(chain.Chain)-1 {
				nextEntry := chain.Chain[i+1]
				if nextEntry.PreviousHash == entry.Hash {
					fmt.Printf("                ✓ Used as previous_hash for Entry %d\n", i+2)
				} else {
					fmt.Printf("                ✗ Next entry's previous_hash does NOT match!\n")
				}
			}
			
			fmt.Printf("  Signature:     %s...\n", entry.Signature[:min(64, len(entry.Signature))])
			if entry.RaftIndex > 0 {
				fmt.Printf("  Raft Index:    %d (Term: %d)\n", entry.RaftIndex, entry.RaftTerm)
			}
			fmt.Println()
		}
		
	case "delete-all":
		err := client.DeleteAllKeys()
		if err != nil {
			log.Fatalf("Failed to delete all keys: %v", err)
		}
		fmt.Println("✅ All keys deleted successfully")
		
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("HSM Client - Manage LMS keys on HSM server")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ./hsm-client <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  help              Show this help message")
	fmt.Println("  generate          Generate a new LMS key")
	fmt.Println("  list              List all available keys")
	fmt.Println("  sign              Sign a message with key_id")
	fmt.Println("  query             Query Raft cluster for key_id's last index")
	fmt.Println("  chain             Get full hash chain for key_id from Raft cluster")
	fmt.Println("  delete-all        Delete all keys from HSM server (WARNING: irreversible)")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -server URL       HSM server URL (default: http://localhost:9090)")
	fmt.Println("  -key-id ID        Key ID for generate/sign/query command")
	fmt.Println("  -msg MESSAGE      Message to sign (for sign command)")
	fmt.Println("  -raft URL         Raft cluster endpoint (for query command, default: http://localhost:8080)")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ./hsm-client help")
	fmt.Println("  ./hsm-client generate")
	fmt.Println("  ./hsm-client generate -key-id my_key")
	fmt.Println("  ./hsm-client list")
	fmt.Println("  ./hsm-client list -server http://159.69.23.29:9090")
	fmt.Println("  ./hsm-client sign -key-id my_key -msg 'hello world' -server http://159.69.23.29:9090")
	fmt.Println("  ./hsm-client query -key-id my_key -raft http://159.69.23.29:8080")
	fmt.Println("  ./hsm-client chain -key-id my_key -raft http://159.69.23.29:8080")
	fmt.Println("  ./hsm-client delete-all -server http://159.69.23.29:9090")
}

