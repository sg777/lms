package main

import (
	"fmt"
	"log"
	
	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

func main() {
	fmt.Println("=== Full LMS Sign/Verify Test ===")
	
	// Test parameters: Single level LMS
	levels := 1
	lmType := []int{5}  // LMS_SHA256_M32_H5
	otsType := []int{1} // LMOTS_SHA256_N32_W1
	
	fmt.Printf("1. Generating key pair...\n")
	privKey, pubKey, err := lms_wrapper.GenerateKeyPair(levels, lmType, otsType)
	if err != nil {
		log.Fatalf("Failed to generate key pair: %v", err)
	}
	fmt.Printf("   ✅ Private key: %d bytes\n", len(privKey))
	fmt.Printf("   ✅ Public key:  %d bytes\n", len(pubKey))
	
	// Test message
	message := []byte("Hello, LMS World! This is a test message.")
	fmt.Printf("\n2. Message to sign: %s\n", string(message))
	
	// Load working key
	fmt.Printf("\n3. Loading working key for signing...\n")
	workingKey, err := lms_wrapper.LoadWorkingKey(privKey, levels, lmType, otsType, 0)
	if err != nil {
		log.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	fmt.Printf("   ✅ Working key loaded\n")
	
	// Sign message
	fmt.Printf("\n4. Signing message...\n")
	signature, err := workingKey.GenerateSignature(message)
	if err != nil {
		log.Fatalf("Failed to sign message: %v", err)
	}
	fmt.Printf("   ✅ Signature generated: %d bytes\n", len(signature))
	fmt.Printf("   Signature (hex): %x\n", signature[:min(64, len(signature))])
	
	// Verify signature
	fmt.Printf("\n5. Verifying signature...\n")
	valid, err := lms_wrapper.VerifySignature(pubKey, message, signature)
	if err != nil {
		log.Fatalf("Failed to verify signature: %v", err)
	}
	
	if valid {
		fmt.Printf("   ✅ Signature is VALID!\n")
	} else {
		fmt.Printf("   ❌ Signature is INVALID!\n")
	}
	
	// Test with wrong message
	fmt.Printf("\n6. Testing with wrong message...\n")
	wrongMessage := []byte("Wrong message!")
	valid, err = lms_wrapper.VerifySignature(pubKey, wrongMessage, signature)
	if err != nil {
		log.Fatalf("Failed to verify signature: %v", err)
	}
	if !valid {
		fmt.Printf("   ✅ Correctly rejected wrong message!\n")
	} else {
		fmt.Printf("   ❌ ERROR: Accepted wrong message!\n")
	}
	
	// Sign second message (stateful - index increments)
	fmt.Printf("\n7. Signing second message (index should increment)...\n")
	message2 := []byte("Second message")
	signature2, err := workingKey.GenerateSignature(message2)
	if err != nil {
		log.Fatalf("Failed to sign second message: %v", err)
	}
	fmt.Printf("   ✅ Second signature: %d bytes\n", len(signature2))
	
	valid2, err := lms_wrapper.VerifySignature(pubKey, message2, signature2)
	if err != nil {
		log.Fatalf("Failed to verify second signature: %v", err)
	}
	if valid2 {
		fmt.Printf("   ✅ Second signature is VALID!\n")
	} else {
		fmt.Printf("   ❌ Second signature is INVALID!\n")
	}
	
	fmt.Println("\n=== All tests passed! ===")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
