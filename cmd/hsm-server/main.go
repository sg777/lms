package main

import (
	"flag"
	"log"
	"strings"

	"github.com/verifiable-state-chains/lms/hsm_server"
)

func main() {
	port := flag.Int("port", 9090, "HSM server port")
	raftEndpointsStr := flag.String("raft-endpoints", "http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080", "Comma-separated list of Raft cluster endpoints")
	
	// Blockchain configuration (enabled by default)
	blockchainEnabled := flag.Bool("blockchain-enabled", true, "Enable blockchain commits (default: true)")
	blockchainRPCURL := flag.String("blockchain-rpc-url", "http://127.0.0.1:22778", "Verus RPC URL")
	blockchainRPCUser := flag.String("blockchain-rpc-user", "user1172159772", "RPC username")
	blockchainRPCPassword := flag.String("blockchain-rpc-password", "pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da", "RPC password")
	blockchainIdentity := flag.String("blockchain-identity", "sg777z.chips.vrsc@", "Verus identity name")
	
	flag.Parse()

	raftEndpoints := strings.Split(*raftEndpointsStr, ",")
	for i := range raftEndpoints {
		raftEndpoints[i] = strings.TrimSpace(raftEndpoints[i])
	}

	// Setup blockchain config (enabled by default)
	var blockchainConfig *hsm_server.BlockchainConfig
	if *blockchainEnabled {
		blockchainConfig = &hsm_server.BlockchainConfig{
			Enabled:      true,
			RPCURL:       *blockchainRPCURL,
			RPCUser:      *blockchainRPCUser,
			RPCPassword:  *blockchainRPCPassword,
			IdentityName: *blockchainIdentity,
		}
		log.Printf("Blockchain commits: ENABLED (identity=%s, rpc=%s)", *blockchainIdentity, *blockchainRPCURL)
	} else {
		log.Printf("Blockchain commits: DISABLED (use -blockchain-enabled=true to enable)")
	}

	server, err := hsm_server.NewHSMServer(*port, raftEndpoints, blockchainConfig)
	if err != nil {
		log.Fatalf("Failed to create HSM server: %v", err)
	}

	log.Printf("Starting HSM server on port %d", *port)
	log.Printf("Every index commit will go to BOTH Raft and Verus blockchain (if enabled)")
	
	if err := server.Start(); err != nil {
		log.Fatalf("HSM server error: %v", err)
	}
}
