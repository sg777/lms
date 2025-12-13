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
	flag.Parse()

	raftEndpoints := strings.Split(*raftEndpointsStr, ",")
	for i := range raftEndpoints {
		raftEndpoints[i] = strings.TrimSpace(raftEndpoints[i])
	}

	server := explorer.NewExplorerServer(*port, raftEndpoints)
	
	log.Printf("Starting LMS Hash Chain Explorer on port %d", *port)
	if err := server.Start(); err != nil {
		log.Fatalf("Explorer server error: %v", err)
	}
}

