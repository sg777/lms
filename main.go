package main

import (
	"bufio"
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
			// Get logs from FSM
			count := hashChainFSM.GetLogCount()
			fmt.Printf("Total log entries: %d\n", count)
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
			for _, node := range cfg.ClusterNodes {
				if string(leaderAddr) == node.Address {
					leaderID = node.ID
					break
				}
			}
			
			// Try to forward via HTTP API
			leaderAPIAddr := ""
			for _, node := range cfg.ClusterNodes {
				if string(leaderAddr) == node.Address {
					ip := node.Address[:len(node.Address)-5] // Remove ":7000"
					leaderAPIAddr = fmt.Sprintf("http://%s:%d", ip, node.APIPort)
					break
				}
			}
			
			if leaderAPIAddr != "" {
				fmt.Printf("⚠️  Not the leader (leader is %s). Forwarding to leader...\n", leaderID)
				// For now, just tell user to use leader or we could implement HTTP forwarding
				fmt.Printf("💡 Tip: Send messages from the leader node (%s) or use HTTP API: %s\n", leaderID, leaderAPIAddr)
				continue
			} else {
				fmt.Printf("⚠️  Not the leader (leader is %s). Cannot forward automatically.\n", leaderAddr)
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

