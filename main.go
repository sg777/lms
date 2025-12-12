package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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

	// Create hash-chain FSM
	hashChainFSM := fsm.NewHashChainFSM(*genesisHash)

	// Create and start service
	svc, err := service.NewService(cfg, hashChainFSM)
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
					
					if messages, ok := result["messages"].([]interface{}); ok {
						fmt.Printf("\n=== All Messages (%d total) ===\n", len(messages))
						for i, msg := range messages {
							fmt.Printf("%d. %s\n", i+1, msg)
						}
						fmt.Printf("=============================\n\n")
					} else {
						fmt.Printf("Total log entries: %.0f\n", result["total_count"].(float64))
					}
				} else {
					fmt.Printf("⚠️  Cannot determine leader API address\n")
				}
			} else if isLeader {
				// We're the leader - get directly from FSM
				messages := hashChainFSM.GetSimpleMessages()
				count := hashChainFSM.GetLogCount()
				
				fmt.Printf("\n=== All Messages (%d total) ===\n", count)
				for i, msg := range messages {
					fmt.Printf("%d. %s\n", i+1, msg)
				}
				fmt.Printf("=============================\n\n")
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

		// Check if this node is the leader
		isLeader := (raft.State().String() == "Leader")
		if !isLeader {
			// Not the leader - forward to leader via HTTP API
			leaderID := ""
			leaderAPIAddr := ""
			for _, node := range cfg.ClusterNodes {
				if string(leaderAddr) == node.Address {
					leaderID = node.ID
					ip := node.Address[:len(node.Address)-5] // Remove ":7000"
					leaderAPIAddr = fmt.Sprintf("http://%s:%d", ip, node.APIPort)
					break
				}
			}
			
			if leaderAPIAddr != "" {
				fmt.Printf("📤 Forwarding message to leader (%s)...\n", leaderID)
				
				// Create a simple message request (as JSON for the API)
				// For now, we'll send it as a simple string via a custom endpoint
				// Or we can use the propose endpoint with a simple format
				
				// Actually, let's use Raft directly but through the leader's Raft instance
				// Wait, we can't access leader's Raft from here. We need HTTP API.
				
				// Create request body - send as simple string message
				reqBody := map[string]interface{}{
					"message": input,
				}
				jsonData, err := json.Marshal(reqBody)
				if err != nil {
					fmt.Printf("❌ Failed to encode message: %v\n", err)
					continue
				}
				
				// Send to leader's /send endpoint (we'll create this)
				resp, err := http.Post(leaderAPIAddr+"/send", "application/json", bytes.NewBuffer(jsonData))
				if err != nil {
					fmt.Printf("❌ Failed to forward to leader: %v\n", err)
					fmt.Printf("💡 Leader is %s at %s\n", leaderID, leaderAPIAddr)
					continue
				}
				defer resp.Body.Close()
				
				if resp.StatusCode == http.StatusOK {
					var result map[string]interface{}
					if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
						fmt.Printf("✅ Message forwarded and applied: %v\n", result)
					} else {
						fmt.Printf("✅ Message forwarded to leader\n")
					}
				} else {
					body, _ := io.ReadAll(resp.Body)
					fmt.Printf("❌ Leader returned error: %s\n", string(body))
				}
				continue
			} else {
				fmt.Printf("⚠️  Not the leader (leader is %s). Cannot determine leader address.\n", leaderAddr)
				continue
			}
		}

		// This node is the leader - apply directly
		fmt.Printf("📤 Sending message to cluster...\n")
		future := raft.Apply([]byte(input), 5*time.Second)
		if err := future.Error(); err != nil {
			fmt.Printf("❌ Failed to apply log: %v\n", err)
		} else {
			fmt.Printf("✅ Applied: %v\n", future.Response())
		}
	}

	// Shutdown
	if err := svc.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}

