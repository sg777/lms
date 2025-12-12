package main

import (
	"fmt"
	"log"
	
	"github.com/verifiable-state-chains/lms/hsm_client"
)

func main() {
	// Connect to HSM server
	serverURL := "http://localhost:9090"
	client := hsm_client.NewHSMClient(serverURL)
	
	// Generate a key (server will generate key_id)
	fmt.Println("Generating LMS key...")
	key1, err := client.GenerateKey("")
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}
	fmt.Printf("Generated key: key_id=%s, index=%d\n", key1.KeyID, key1.Index)
	
	// Generate another key with specific key_id
	fmt.Println("\nGenerating LMS key with specific key_id...")
	key2, err := client.GenerateKey("my_custom_key")
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}
	fmt.Printf("Generated key: key_id=%s, index=%d\n", key2.KeyID, key2.Index)
	
	// List all keys
	fmt.Println("\nListing all keys...")
	keys, err := client.ListKeys()
	if err != nil {
		log.Fatalf("Failed to list keys: %v", err)
	}
	
	fmt.Printf("Total keys: %d\n", len(keys))
	for i, key := range keys {
		fmt.Printf("  %d. key_id=%s, index=%d, created=%s\n", 
			i+1, key.KeyID, key.Index, key.Created)
	}
}

