package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/verifiable-state-chains/lms/explorer"
)

func main() {
	port := flag.Int("port", 8081, "Explorer server port")
	raftEndpointsStr := flag.String("raft-endpoints", "http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080", "Comma-separated list of Raft cluster endpoints")
	hsmEndpoint := flag.String("hsm-endpoint", "http://159.69.23.31:9090", "HSM server endpoint")
	logFile := flag.String("log-file", "", "Optional: Write logs to file (default: stdout/stderr)")
	flag.Parse()

	// Set up logging
	if *logFile != "" {
		logFileHandle, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file %s: %v", *logFile, err)
		}
		defer logFileHandle.Close()
		log.SetOutput(logFileHandle)
		log.Printf("========================================")
		log.Printf("Logging to file: %s", *logFile)
		log.Printf("========================================")
	}

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

