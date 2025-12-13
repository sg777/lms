package main

import (
	"bytes"
	"fmt"
	"log"
	
	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

func main() {
	fmt.Println("=== Full LMS Sign/Verify Test ===")
	
	// Standard parameters: h=5, w=1 (LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W1)
	// This gives us ~32 signatures maximum
	levels := 1
	lmType := []int{lms_wrapper.LMS_SHA256_M32_H5}  // h=5
	otsType := []int{lms_wrapper.LMOTS_SHA256_N32_W1} // w=1
	
	// Display parameter information
	paramDesc := lms_wrapper.FormatParameterSet(levels, lmType, otsType)
	fmt.Printf("Parameters: %s\n", paramDesc)
	fmt.Printf("  h (height) = %d\n", lms_wrapper.GetLMSHeight(lmType[0]))
	fmt.Printf("  w (Winternitz) = %d\n", lms_wrapper.GetOTSW(otsType[0]))
	fmt.Printf("  Max signatures = %d\n\n", lms_wrapper.GetMaxSignatures(lmType[0]))
	
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
	
	// Verify signature length matches expected
	expectedSigLen, err := lms_wrapper.GetSignatureLen(levels, lmType, otsType)
	if err != nil {
		log.Fatalf("Failed to get expected signature length: %v", err)
	}
	
	fmt.Printf("   ✅ Signature generated: %d bytes\n", len(signature))
	fmt.Printf("   Expected length: %d bytes\n", expectedSigLen)
	if len(signature) != expectedSigLen {
		log.Fatalf("   ❌ Signature length mismatch! Got %d, expected %d", len(signature), expectedSigLen)
	}
	fmt.Printf("   ✅ Signature length verified!\n")
	
	// Display signature (first 128 bytes in hex)
	fmt.Printf("   Signature (first 128 bytes, hex):\n")
	maxDisplay := 128
	if len(signature) < maxDisplay {
		maxDisplay = len(signature)
	}
	fmt.Printf("   %x\n", signature[:maxDisplay])
	if len(signature) > maxDisplay {
		fmt.Printf("   ... (%d more bytes)\n", len(signature)-maxDisplay)
	}
	
	// Verify signature cryptographically
	fmt.Printf("\n5. Verifying signature cryptographically...\n")
	valid, err := lms_wrapper.VerifySignature(pubKey, message, signature)
	if err != nil {
		log.Fatalf("   ❌ Failed to verify signature: %v", err)
	}
	
	if valid {
		fmt.Printf("   ✅ Signature is CRYPTOGRAPHICALLY VALID!\n")
		fmt.Printf("   ✅ Signature correctly authenticates the message\n")
		fmt.Printf("   ✅ Signature correctly authenticates the public key\n")
	} else {
		log.Fatalf("   ❌ Signature is CRYPTOGRAPHICALLY INVALID!")
	}
	
	// Verify signature is not all zeros
	allZeros := true
	for _, b := range signature {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		log.Fatalf("   ❌ ERROR: Signature is all zeros!")
	} else {
		fmt.Printf("   ✅ Signature contains valid cryptographic data\n")
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
		log.Fatalf("   ❌ ERROR: Accepted wrong message!")
	}
	
	// Sign second message (stateful - index increments)
	fmt.Printf("\n7. Signing second message (index should increment)...\n")
	message2 := []byte("Second message")
	signature2, err := workingKey.GenerateSignature(message2)
	if err != nil {
		log.Fatalf("Failed to sign second message: %v", err)
	}
	fmt.Printf("   ✅ Second signature: %d bytes\n", len(signature2))
	
	if len(signature2) != expectedSigLen {
		log.Fatalf("   ❌ Second signature length mismatch! Got %d, expected %d", len(signature2), expectedSigLen)
	}
	
	valid2, err := lms_wrapper.VerifySignature(pubKey, message2, signature2)
	if err != nil {
		log.Fatalf("Failed to verify second signature: %v", err)
	}
	if valid2 {
		fmt.Printf("   ✅ Second signature is CRYPTOGRAPHICALLY VALID!\n")
	} else {
		log.Fatalf("   ❌ Second signature is INVALID!")
	}
	
	// Verify signatures are different (stateful scheme)
	if bytes.Equal(signature, signature2) {
		log.Fatalf("   ❌ ERROR: Signatures are identical (stateful scheme should produce different signatures)")
	} else {
		fmt.Printf("   ✅ Signatures are different (correct stateful behavior)\n")
	}
	
	fmt.Println("\n=== All tests passed! ===")
}
