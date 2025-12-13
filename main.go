package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/verifiable-state-chains/lms/fsm"
	"github.com/verifiable-state-chains/lms/service"
)

func main() {
	// Parse command-line flags
	nodeID := flag.String("id", "node1", "Node ID (e.g., node1, node2, node3)")
	nodeAddr := flag.String("addr", "159.69.23.29:7000", "Node address (IP:port for Raft)")
	apiPort := flag.Int("api-port", 8080, "API server port")
	raftDir := flag.String("raft-dir", "./raft-data", "Raft data directory")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster")
	genesisHash := flag.String("genesis-hash", "lms_genesis_hash_verifiable_state_chains", "Genesis hash for the chain")
	flag.Parse()

	// Create configuration
	cfg := service.DefaultConfig()
	cfg.NodeID = *nodeID
	cfg.NodeAddr = *nodeAddr
	cfg.APIPort = *apiPort
	cfg.RaftDir = *raftDir
	cfg.Bootstrap = *bootstrap

	// Create combined FSM (hash-chain + key-index)
	// Attestation public key path: ./keys/attestation_public_key.pem
	var svc *service.Service
	var fsmInstance service.FSMInterface
	combinedFSM, err := fsm.NewCombinedFSM(*genesisHash, "./keys/attestation_public_key.pem")
	if err != nil {
		log.Printf("Warning: Failed to load attestation public key, continuing without signature verification: %v", err)
		// Fallback to hash-chain only if key not found
		hashChainFSM := fsm.NewHashChainFSM(*genesisHash)
		fsmInstance = hashChainFSM
		svc, err = service.NewService(cfg, hashChainFSM)
	} else {
		// Create and start service with combined FSM
		fsmInstance = combinedFSM
		svc, err = service.NewService(cfg, combinedFSM)
	}
	
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v, shutting down...", sig)
		if err := svc.Shutdown(); err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		os.Exit(0)
	}()

	// Start service in background
	go func() {
		log.Printf("Starting Verifiable State Chains service")
		log.Printf("  Node ID: %s", *nodeID)
		log.Printf("  Raft Address: %s", *nodeAddr)
		log.Printf("  API Port: %d", *apiPort)
		log.Printf("  Bootstrap: %v", *bootstrap)
		log.Printf("  Genesis Hash: %s", *genesisHash)

		if err := svc.Start(); err != nil {
			log.Fatalf("Service error: %v", err)
		}
	}()

	// Wait a bit for service to start
	time.Sleep(2 * time.Second)

	// Add simple CLI interface like the working code
	raft := svc.GetRaft()
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("\n=== LMS Service CLI ===\n")
	fmt.Printf("Node %s running. Enter commands:\n", *nodeID)
	fmt.Printf("  - Type a message to send to cluster\n")
	fmt.Printf("  - Type 'list' to see all logs\n")
	fmt.Printf("  - Type 'health' to check status\n")
	fmt.Printf("  - Type 'exit' to quit\n\n")

	for scanner.Scan() {
		input := scanner.Text()
		if input == "exit" {
			break
		}
		if input == "list" {
			// Check if we're leader or need to query leader
			leaderAddr := raft.Leader()
			isLeader := (raft.State().String() == "Leader")
			
			if !isLeader && leaderAddr != "" {
				// Not leader - query leader via HTTP API
				leaderAPIAddr := ""
				for _, node := range cfg.ClusterNodes {
					if string(leaderAddr) == node.Address {
						ip := node.Address[:len(node.Address)-5] // Remove ":7000"
						leaderAPIAddr = fmt.Sprintf("http://%s:%d", ip, node.APIPort)
						break
					}
				}
				
				if leaderAPIAddr != "" {
					fmt.Printf("📋 Querying leader for logs...\n")
					resp, err := http.Get(leaderAPIAddr + "/list")
					if err != nil {
						fmt.Printf("❌ Failed to query leader: %v\n", err)
						continue
					}
					defer resp.Body.Close()
					
					var result map[string]interface{}
					if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
						fmt.Printf("❌ Failed to decode response: %v\n", err)
						continue
					}
					
					// Display hash chain information from API response
					genesisHash := ""
					if gh, ok := result["genesis_hash"].(string); ok {
						genesisHash = gh
					}
					
					if logEntries, ok := result["log_entries"].([]interface{}); ok && len(logEntries) > 0 {
						fmt.Printf("\n=== Committed Log Entries (%d total) ===\n", len(logEntries))
						if genesisHash != "" {
							fmt.Printf("Genesis Hash: %s\n\n", genesisHash)
						}
						
						for i, entryData := range logEntries {
							entry, ok := entryData.(map[string]interface{})
							if !ok {
								continue
							}
							
							index := uint64(0)
							term := uint64(0)
							if idx, ok := entry["index"].(float64); ok {
								index = uint64(idx)
							}
							if t, ok := entry["term"].(float64); ok {
								term = uint64(t)
							}
							
							fmt.Printf("--- Entry %d (Raft Index: %d, Term: %d) ---\n", i+1, index, term)
							
							// Check if entry has hash chain info in the response
							if prevHash, ok := entry["previous_hash"].(string); ok {
								fmt.Printf("  Previous Hash: %s\n", prevHash)
								if prevHash == genesisHash {
									fmt.Printf("  [GENESIS LINK]\n")
								}
							}
							if hash, ok := entry["hash"].(string); ok {
								fmt.Printf("  Current Hash:  %s\n", hash)
							}
							if lmsIdx, ok := entry["lms_index"].(float64); ok {
								fmt.Printf("  LMS Index:     %d\n", uint64(lmsIdx))
							}
							if seq, ok := entry["sequence_number"].(float64); ok {
								fmt.Printf("  Sequence:      %d\n", uint64(seq))
							}
							if msgHash, ok := entry["message_signed"].(string); ok {
								fmt.Printf("  Message Hash:  %s\n", msgHash)
							}
							if entryType, ok := entry["type"].(string); ok && entryType == "simple_message" {
								fmt.Printf("  Type: Simple Message\n")
							}
							
							fmt.Printf("\n")
						}
						fmt.Printf("==========================================\n\n")
					} else if messages, ok := result["messages"].([]interface{}); ok && len(messages) > 0 {
						// Fallback to simple messages
						fmt.Printf("\n=== All Messages (%d total) ===\n", len(messages))
						if genesisHash != "" {
							fmt.Printf("Genesis Hash: %s\n\n", genesisHash)
						}
						for i, msg := range messages {
							fmt.Printf("%d. %s\n", i+1, msg)
						}
						fmt.Printf("=============================\n\n")
					} else {
						count := 0
						if tc, ok := result["total_count"].(float64); ok {
							count = int(tc)
						}
						fmt.Printf("\n=== Committed Log Entries (%d total) ===\n", count)
						if genesisHash != "" {
							fmt.Printf("Genesis Hash: %s\n", genesisHash)
						}
						fmt.Printf("(No entries to display)\n")
						fmt.Printf("==========================================\n\n")
					}
				} else {
					fmt.Printf("⚠️  Cannot determine leader API address\n")
				}
			} else if isLeader {
				// We're the leader - get directly from FSM
				logEntries := fsmInstance.GetAllLogEntries()
				genesisHash := fsmInstance.GetGenesisHash()
				
				fmt.Printf("\n=== Committed Log Entries (%d total) ===\n", len(logEntries))
				fmt.Printf("Genesis Hash: %s\n\n", genesisHash)
				
				for i, entry := range logEntries {
					fmt.Printf("--- Entry %d (Raft Index: %d, Term: %d) ---\n", i+1, entry.Index, entry.Term)
					
					if entry.Attestation != nil {
						// It's an attestation - show hash chain info
						payload, err := entry.Attestation.GetChainedPayload()
						if err == nil {
							fmt.Printf("  Previous Hash: %s\n", payload.PreviousHash)
							fmt.Printf("  LMS Index:     %d\n", payload.LMSIndex)
							fmt.Printf("  Sequence:      %d\n", payload.SequenceNumber)
							fmt.Printf("  Message Hash:  %s\n", payload.MessageSigned)
							fmt.Printf("  Timestamp:     %s\n", payload.Timestamp)
						}
						
						// Compute current hash
						hash, err := entry.Attestation.ComputeHash()
						if err == nil {
							fmt.Printf("  Current Hash:  %s\n", hash)
						}
						
						// Show if this links to genesis
						if payload != nil && payload.PreviousHash == genesisHash {
							fmt.Printf("  [GENESIS LINK]\n")
						}
					} else {
						// Simple string message
						fmt.Printf("  Type: Simple Message\n")
						if i < len(fsmInstance.GetSimpleMessages()) {
							msgs := fsmInstance.GetSimpleMessages()
							fmt.Printf("  Content: %s\n", msgs[i])
						}
					}
					fmt.Printf("\n")
				}
				fmt.Printf("==========================================\n\n")
			} else {
				fmt.Printf("⚠️  No leader available\n")
			}
			continue
		}
		if input == "health" {
			leader := raft.Leader()
			state := raft.State()
			isLeader := (raft.State().String() == "Leader")
			fmt.Printf("State: %s, Leader: %s, Is Leader: %v\n", state, leader, isLeader)
			continue
		}

		// Check if current node is leader
		leaderAddr := raft.Leader()
		if leaderAddr == "" {
			fmt.Println("⚠️  No leader yet, waiting for leader election...")
			continue
		}

		// SECURITY: Direct commits from CLI are DISABLED
		// Only /commit_index endpoint with EC signature authentication is allowed
		// This service only accepts LMS index-related messages from HSM server
		fmt.Printf("❌ SECURITY: Direct commits are not allowed.\n")
		fmt.Printf("   This service only accepts LMS index-related messages.\n")
		fmt.Printf("   Use /commit_index endpoint with proper EC signature authentication.\n")
		fmt.Printf("   Only HSM server with attestation private key can commit.\n")
		continue
	}

	// Shutdown
	if err := svc.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}

