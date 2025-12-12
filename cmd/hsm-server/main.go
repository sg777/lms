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
	flag.Parse()
	
	raftEndpoints := strings.Split(*raftEndpointsStr, ",")
	for i := range raftEndpoints {
		raftEndpoints[i] = strings.TrimSpace(raftEndpoints[i])
	}
	
	server, err := hsm_server.NewHSMServer(*port, raftEndpoints)
	if err != nil {
		log.Fatalf("Failed to create HSM server: %v", err)
	}
	
	log.Printf("Starting HSM Server on port %d", *port)
	if err := server.Start(); err != nil {
		log.Fatalf("HSM Server error: %v", err)
	}
}

