package main

import (
	"fmt"
	"os"
	"path/filepath"
	
	"github.com/verifiable-state-chains/lms/hsm_server"
)

func main() {
	fmt.Printf("Current directory: %s\n", getCurrentDir())
	fmt.Printf("Keys directory: ./keys\n")
	
	privKey, pubKey, err := hsm_server.LoadOrGenerateAttestationKeyPair()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	
	fmt.Printf("✅ Keys loaded successfully\n")
	fmt.Printf("Public key X: %x\n", pubKey.X.Bytes())
	fmt.Printf("Public key Y: %x\n", pubKey.Y.Bytes())
	
	// Check if keys file exists
	keysPath := filepath.Join("./keys", "attestation_public_key.pem")
	if _, err := os.Stat(keysPath); err == nil {
		fmt.Printf("✅ Keys file exists: %s\n", keysPath)
	} else {
		fmt.Printf("❌ Keys file NOT found: %s\n", keysPath)
	}
}

func getCurrentDir() string {
	dir, _ := os.Getwd()
	return dir
}

