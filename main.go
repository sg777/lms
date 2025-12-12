package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/verifiable-state-chains/lms/fsm"
	"github.com/verifiable-state-chains/lms/service"
)

func main() {
	// Parse command-line flags
	nodeID := flag.String("id", "node1", "Node ID (e.g., node1, node2, node3)")
	nodeAddr := flag.String("addr", "159.69.23.29:7000", "Node address (IP:port for Raft)")
	apiPort := flag.Int("api-port", 8080, "API server port")
	raftPort := flag.Int("raft-port", 7000, "Raft transport port")
	raftDir := flag.String("raft-dir", "./raft-data", "Raft data directory")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster")
	genesisHash := flag.String("genesis-hash", "lms_genesis_hash_verifiable_state_chains", "Genesis hash for the chain")
	flag.Parse()

	// Create configuration
	cfg := service.DefaultConfig()
	cfg.NodeID = *nodeID
	cfg.NodeAddr = *nodeAddr
	cfg.APIPort = *apiPort
	cfg.RaftPort = *raftPort
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

	// Start service (blocks)
	log.Printf("Starting Verifiable State Chains service")
	log.Printf("  Node ID: %s", *nodeID)
	log.Printf("  Raft Address: %s", *nodeAddr)
	log.Printf("  API Port: %d", *apiPort)
	log.Printf("  Raft Port: %d", *raftPort)
	log.Printf("  Bootstrap: %v", *bootstrap)
	log.Printf("  Genesis Hash: %s", *genesisHash)

	if err := svc.Start(); err != nil {
		log.Fatalf("Service error: %v", err)
	}
}

