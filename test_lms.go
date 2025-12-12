package main

import (
	"fmt"
	"log"
	
	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

func main() {
	fmt.Println("=== LMS/HSS Key Generation and Verification Test ===")
	
	// Test parameters: Single level LMS (not HSS)
	// Using LMS_SHA256_M32_H5 (parameter set 5) and LMOTS_SHA256_N32_W1 (parameter set 1)
	levels := 1
	lmType := []int{5}  // LMS_SHA256_M32_H5
	otsType := []int{1} // LMOTS_SHA256_N32_W1
	
	fmt.Printf("Generating key pair with levels=%d, lm_type=%v, ots_type=%v\n", levels, lmType, otsType)
	
	// Generate key pair
	privKey, pubKey, err := lms_wrapper.GenerateKeyPair(levels, lmType, otsType)
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}
	
	fmt.Printf("✅ Key pair generated successfully!\n")
	fmt.Printf("   Private key length: %d bytes\n", len(privKey))
	fmt.Printf("   Public key length:  %d bytes\n", len(pubKey))
	fmt.Printf("   Public key (hex):   %x\n", pubKey[:min(32, len(pubKey))])
	
	fmt.Println("\n✅ Key generation works!")
}
