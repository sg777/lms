package main

import (
	"flag"
	"log"
	"strings"

	"github.com/verifiable-state-chains/lms/explorer"
)

func main() {
	port := flag.Int("port", 8081, "Explorer server port")
	raftEndpointsStr := flag.String("raft-endpoints", "http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080", "Comma-separated list of Raft cluster endpoints")
	hsmEndpoint := flag.String("hsm-endpoint", "http://127.0.0.1:9090", "HSM server endpoint (required - explorer cannot function without HSM server)")
	flag.Parse()

	raftEndpoints := strings.Split(*raftEndpointsStr, ",")
	for i := range raftEndpoints {
		raftEndpoints[i] = strings.TrimSpace(raftEndpoints[i])
	}

	server, err := explorer.NewExplorerServer(*port, raftEndpoints, *hsmEndpoint)
	if err != nil {
		log.Fatalf("Failed to create explorer server: %v", err)
	}
	
	log.Printf("Starting LMS Hash Chain Explorer on port %d", *port)
	log.Printf("HSM endpoint: %s", *hsmEndpoint)
	if err := server.Start(); err != nil {
		log.Fatalf("Explorer server error: %v", err)
	}
}

