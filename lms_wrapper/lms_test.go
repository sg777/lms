package lms_wrapper

import (
	"bytes"
	"testing"
)

// Test parameter sets with standard values: h=5, w=1
var (
	standardLevels = 1
	standardLmType = []int{LMS_SHA256_M32_H5}  // h=5
	standardOtsType = []int{LMOTS_SHA256_N32_W1} // w=1
)

func TestParameterConstants(t *testing.T) {
	// Test height extraction
	h := GetLMSHeight(LMS_SHA256_M32_H5)
	if h != 5 {
		t.Errorf("Expected height 5, got %d", h)
	}
	
	h = GetLMSHeight(LMS_SHA256_M32_H10)
	if h != 10 {
		t.Errorf("Expected height 10, got %d", h)
	}
	
	// Test Winternitz parameter extraction
	w := GetOTSW(LMOTS_SHA256_N32_W1)
	if w != 1 {
		t.Errorf("Expected w=1, got %d", w)
	}
	
	w = GetOTSW(LMOTS_SHA256_N32_W8)
	if w != 8 {
		t.Errorf("Expected w=8, got %d", w)
	}
	
	// Test max signatures
	maxSigs := GetMaxSignatures(LMS_SHA256_M32_H5)
	if maxSigs != 32 {
		t.Errorf("Expected 32 signatures for h=5, got %d", maxSigs)
	}
	
	maxSigs = GetMaxSignatures(LMS_SHA256_M32_H10)
	if maxSigs != 1024 {
		t.Errorf("Expected 1024 signatures for h=10, got %d", maxSigs)
	}
}

func TestFormatParameterSet(t *testing.T) {
	desc := FormatParameterSet(standardLevels, standardLmType, standardOtsType)
	expected := "LMS: h=5, w=1 (max 32 signatures)"
	if desc != expected {
		t.Errorf("Expected '%s', got '%s'", expected, desc)
	}
}

func TestGetSignatureLen(t *testing.T) {
	sigLen, err := GetSignatureLen(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to get signature length: %v", err)
	}
	
	// For LMS_SHA256_M32_H5 with LMOTS_SHA256_N32_W1, signature should be 8688 bytes
	if sigLen != 8688 {
		t.Errorf("Expected signature length 8688, got %d", sigLen)
	}
}

func TestGetPublicKeyLen(t *testing.T) {
	pubKeyLen, err := GetPublicKeyLen(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to get public key length: %v", err)
	}
	
	// For LMS_SHA256_M32_H5 with LMOTS_SHA256_N32_W1, public key should be 60 bytes
	if pubKeyLen != 60 {
		t.Errorf("Expected public key length 60, got %d", pubKeyLen)
	}
}

func TestGenerateKeyPair(t *testing.T) {
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	if len(privKey) == 0 {
		t.Error("Private key is empty")
	}
	
	if len(pubKey) == 0 {
		t.Error("Public key is empty")
	}
	
	// Verify public key length matches expected
	expectedPubKeyLen, err := GetPublicKeyLen(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to get expected public key length: %v", err)
	}
	
	if len(pubKey) != expectedPubKeyLen {
		t.Errorf("Public key length mismatch: expected %d, got %d", expectedPubKeyLen, len(pubKey))
	}
}

func TestLoadWorkingKey(t *testing.T) {
	privKey, _, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	if workingKey == nil {
		t.Error("Working key is nil")
	}
}

func TestSignAndVerify(t *testing.T) {
	// Generate key pair
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	// Load working key
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	// Test message
	message := []byte("Test message for LMS signature")
	
	// Sign message
	signature, err := workingKey.GenerateSignature(message)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}
	
	// Verify signature length
	expectedSigLen, err := GetSignatureLen(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to get expected signature length: %v", err)
	}
	
	if len(signature) != expectedSigLen {
		t.Errorf("Signature length mismatch: expected %d, got %d", expectedSigLen, len(signature))
	}
	
	// Verify signature is not all zeros or empty
	if len(signature) == 0 {
		t.Error("Signature is empty")
	}
	
	allZeros := true
	for _, b := range signature {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		t.Error("Signature should not be all zeros")
	}
	
	// Verify signature cryptographically
	valid, err := VerifySignature(pubKey, message, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	
	if !valid {
		t.Error("Signature verification failed - signature is cryptographically invalid")
	}
	
	// Verify again (should still be valid)
	valid2, err := VerifySignature(pubKey, message, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature second time: %v", err)
	}
	if !valid2 {
		t.Error("Signature should remain valid on second verification")
	}
}

func TestSignAndVerifyWrongMessage(t *testing.T) {
	// Generate key pair
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	// Load working key
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	// Sign original message
	message := []byte("Original message")
	signature, err := workingKey.GenerateSignature(message)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}
	
	// First verify with correct message (should pass)
	valid, err := VerifySignature(pubKey, message, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if !valid {
		t.Error("Signature should be valid for correct message")
	}
	
	// Try to verify with wrong message
	wrongMessage := []byte("Wrong message")
	valid, err = VerifySignature(pubKey, wrongMessage, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	
	if valid {
		t.Error("Signature verification should have failed for wrong message")
	}
	
	// Try with empty message (may return error or fail validation)
	valid, err = VerifySignature(pubKey, []byte(""), signature)
	if err != nil {
		// Error is acceptable for empty message
		return
	}
	if valid {
		t.Error("Signature verification should have failed for empty message")
	}
	
	// Try with similar but different message
	similarMessage := []byte("Original messag") // Missing last 'e'
	valid, err = VerifySignature(pubKey, similarMessage, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if valid {
		t.Error("Signature verification should have failed for similar but different message")
	}
}

func TestStatefulSigning(t *testing.T) {
	// Generate key pair
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	// Load working key
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	// Sign multiple messages (stateful - private key state should change)
	messages := [][]byte{
		[]byte("First message"),
		[]byte("Second message"),
		[]byte("Third message"),
	}
	
	signatures := make([][]byte, len(messages))
	
	for i, msg := range messages {
		sig, err := workingKey.GenerateSignature(msg)
		if err != nil {
			t.Fatalf("Failed to sign message %d: %v", i, err)
		}
		signatures[i] = sig
		
		// Verify each signature
		valid, err := VerifySignature(pubKey, msg, sig)
		if err != nil {
			t.Fatalf("Failed to verify signature %d: %v", i, err)
		}
		if !valid {
			t.Errorf("Signature %d verification failed", i)
		}
	}
	
	// Verify signatures are different (different indices)
	if bytes.Equal(signatures[0], signatures[1]) {
		t.Error("Signatures should be different (stateful scheme)")
	}
	if bytes.Equal(signatures[1], signatures[2]) {
		t.Error("Signatures should be different (stateful scheme)")
	}
}

func TestGetPrivateKeyState(t *testing.T) {
	// Generate key pair
	privKey1, _, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	// Load working key
	workingKey, err := LoadWorkingKey(privKey1, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	// Get initial private key state
	initialState := workingKey.GetPrivateKey()
	if !bytes.Equal(initialState, privKey1) {
		t.Error("Initial private key state should match original private key")
	}
	
	// Sign a message (this should update the private key state)
	message := []byte("Test message")
	_, err = workingKey.GenerateSignature(message)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}
	
	// Get updated private key state
	updatedState := workingKey.GetPrivateKey()
	
	// State should have changed (LMS is stateful)
	if bytes.Equal(initialState, updatedState) {
		t.Error("Private key state should have changed after signing")
	}
}

// Test with different parameter sets
func TestDifferentParameterSets(t *testing.T) {
	testCases := []struct {
		name     string
		levels   int
		lmType   []int
		otsType  []int
		expHeight int
		expW     int
	}{
		{
			name:      "h=5, w=1",
			levels:    1,
			lmType:    []int{LMS_SHA256_M32_H5},
			otsType:   []int{LMOTS_SHA256_N32_W1},
			expHeight: 5,
			expW:      1,
		},
		{
			name:      "h=5, w=2",
			levels:    1,
			lmType:    []int{LMS_SHA256_M32_H5},
			otsType:   []int{LMOTS_SHA256_N32_W2},
			expHeight: 5,
			expW:      2,
		},
		{
			name:      "h=10, w=2",
			levels:    1,
			lmType:    []int{LMS_SHA256_M32_H10},
			otsType:   []int{LMOTS_SHA256_N32_W2},
			expHeight: 10,
			expW:      2,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify parameter extraction
			h := GetLMSHeight(tc.lmType[0])
			if h != tc.expHeight {
				t.Errorf("Expected height %d, got %d", tc.expHeight, h)
			}
			
			w := GetOTSW(tc.otsType[0])
			if w != tc.expW {
				t.Errorf("Expected w=%d, got %d", tc.expW, w)
			}
			
			// Test key generation
			privKey, pubKey, err := GenerateKeyPair(tc.levels, tc.lmType, tc.otsType)
			if err != nil {
				t.Fatalf("Failed to generate key pair: %v", err)
			}
			
			if len(privKey) == 0 || len(pubKey) == 0 {
				t.Error("Key pair should not be empty")
			}
			
			// Test signing and verification
			workingKey, err := LoadWorkingKey(privKey, tc.levels, tc.lmType, tc.otsType, 0)
			if err != nil {
				t.Fatalf("Failed to load working key: %v", err)
			}
			defer workingKey.Free()
			
			message := []byte("Test message")
			signature, err := workingKey.GenerateSignature(message)
			if err != nil {
				t.Fatalf("Failed to sign: %v", err)
			}
			
			valid, err := VerifySignature(pubKey, message, signature)
			if err != nil {
				t.Fatalf("Failed to verify: %v", err)
			}
			if !valid {
				t.Error("Signature should be valid")
			}
		})
	}
}

// TestSignatureTampering verifies that tampered signatures are rejected
func TestSignatureTampering(t *testing.T) {
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	message := []byte("Test message")
	signature, err := workingKey.GenerateSignature(message)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}
	
	// Verify original signature is valid
	valid, err := VerifySignature(pubKey, message, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if !valid {
		t.Error("Original signature should be valid")
	}
	
	// Test: Tamper with signature (flip a bit)
	tamperedSig := make([]byte, len(signature))
	copy(tamperedSig, signature)
	tamperedSig[0] ^= 0x01 // Flip first bit
	
	valid, err = VerifySignature(pubKey, message, tamperedSig)
	if err != nil {
		t.Fatalf("Failed to verify tampered signature: %v", err)
	}
	if valid {
		t.Error("Tampered signature should be rejected")
	}
	
	// Test: Tamper with middle of signature
	copy(tamperedSig, signature)
	tamperedSig[len(tamperedSig)/2] ^= 0xFF // Flip all bits in middle byte
	
	valid, err = VerifySignature(pubKey, message, tamperedSig)
	if err != nil {
		t.Fatalf("Failed to verify tampered signature: %v", err)
	}
	if valid {
		t.Error("Tampered signature should be rejected")
	}
	
	// Test: Tamper with end of signature
	copy(tamperedSig, signature)
	tamperedSig[len(tamperedSig)-1] ^= 0x80 // Flip last bit
	
	valid, err = VerifySignature(pubKey, message, tamperedSig)
	if err != nil {
		t.Fatalf("Failed to verify tampered signature: %v", err)
	}
	if valid {
		t.Error("Tampered signature should be rejected")
	}
	
	// Test: Truncated signature
	truncatedSig := signature[:len(signature)-1]
	valid, err = VerifySignature(pubKey, message, truncatedSig)
	if err != nil {
		// Error is acceptable for truncated signature
		if valid {
			t.Error("Truncated signature should be rejected")
		}
	} else if valid {
		t.Error("Truncated signature should be rejected")
	}
}

// TestSignatureCrossKey verifies signatures can't be verified with wrong public key
func TestSignatureCrossKey(t *testing.T) {
	// Generate first key pair
	privKey1, pubKey1, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate first key pair: %v", err)
	}
	
	// Generate second key pair
	privKey2, pubKey2, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate second key pair: %v", err)
	}
	
	// Sign with first key
	workingKey1, err := LoadWorkingKey(privKey1, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load first working key: %v", err)
	}
	defer workingKey1.Free()
	
	message := []byte("Test message")
	signature1, err := workingKey1.GenerateSignature(message)
	if err != nil {
		t.Fatalf("Failed to sign with first key: %v", err)
	}
	
	// Verify with correct public key (should pass)
	valid, err := VerifySignature(pubKey1, message, signature1)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if !valid {
		t.Error("Signature should be valid with correct public key")
	}
	
	// Try to verify with wrong public key (should fail)
	valid, err = VerifySignature(pubKey2, message, signature1)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if valid {
		t.Error("Signature should not be valid with wrong public key")
	}
	
	// Sign with second key
	workingKey2, err := LoadWorkingKey(privKey2, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load second working key: %v", err)
	}
	defer workingKey2.Free()
	
	signature2, err := workingKey2.GenerateSignature(message)
	if err != nil {
		t.Fatalf("Failed to sign with second key: %v", err)
	}
	
	// Verify second signature with its own key
	valid, err = VerifySignature(pubKey2, message, signature2)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if !valid {
		t.Error("Second signature should be valid with its own public key")
	}
	
	// Verify second signature with first key (should fail)
	valid, err = VerifySignature(pubKey1, message, signature2)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if valid {
		t.Error("Second signature should not be valid with first public key")
	}
	
	// Verify signatures are different (same message, different keys)
	if bytes.Equal(signature1, signature2) {
		t.Error("Signatures from different keys should be different")
	}
}

// TestSignatureReuse verifies that signatures cannot be reused with different messages
func TestSignatureReuse(t *testing.T) {
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	message1 := []byte("First message")
	signature1, err := workingKey.GenerateSignature(message1)
	if err != nil {
		t.Fatalf("Failed to sign first message: %v", err)
	}
	
	message2 := []byte("Second message")
	signature2, err := workingKey.GenerateSignature(message2)
	if err != nil {
		t.Fatalf("Failed to sign second message: %v", err)
	}
	
	// Verify each signature with its own message
	valid, err := VerifySignature(pubKey, message1, signature1)
	if err != nil {
		t.Fatalf("Failed to verify signature 1: %v", err)
	}
	if !valid {
		t.Error("Signature 1 should be valid for message 1")
	}
	
	valid, err = VerifySignature(pubKey, message2, signature2)
	if err != nil {
		t.Fatalf("Failed to verify signature 2: %v", err)
	}
	if !valid {
		t.Error("Signature 2 should be valid for message 2")
	}
	
	// Try to reuse signature1 with message2 (should fail)
	valid, err = VerifySignature(pubKey, message2, signature1)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if valid {
		t.Error("Signature 1 should not be valid for message 2")
	}
	
	// Try to reuse signature2 with message1 (should fail)
	valid, err = VerifySignature(pubKey, message1, signature2)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	if valid {
		t.Error("Signature 2 should not be valid for message 1")
	}
}

// TestMultipleSignatures verifies correctness across multiple signatures
func TestMultipleSignatures(t *testing.T) {
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	// Sign multiple messages
	messages := [][]byte{
		[]byte("Message 1"),
		[]byte("Message 2"),
		[]byte("Message 3"),
		[]byte("Message 4"),
		[]byte("Message 5"),
	}
	
	signatures := make([][]byte, len(messages))
	
	// Generate all signatures
	for i, msg := range messages {
		sig, err := workingKey.GenerateSignature(msg)
		if err != nil {
			t.Fatalf("Failed to sign message %d: %v", i+1, err)
		}
		signatures[i] = sig
	}
	
	// Verify each signature with its correct message
	for i, msg := range messages {
		valid, err := VerifySignature(pubKey, msg, signatures[i])
		if err != nil {
			t.Fatalf("Failed to verify signature %d: %v", i+1, err)
		}
		if !valid {
			t.Errorf("Signature %d verification failed", i+1)
		}
	}
	
	// Verify signatures are all different
	for i := 0; i < len(signatures); i++ {
		for j := i + 1; j < len(signatures); j++ {
			if bytes.Equal(signatures[i], signatures[j]) {
				t.Errorf("Signatures %d and %d should be different (stateful scheme)", i+1, j+1)
			}
		}
	}
	
	// Verify cross-message validation fails (each signature only valid for its message)
	for i := 0; i < len(signatures); i++ {
		for j := 0; j < len(messages); j++ {
			if i != j {
				valid, err := VerifySignature(pubKey, messages[j], signatures[i])
				if err != nil {
					// Error is acceptable
					continue
				}
				if valid {
					t.Errorf("Signature %d should not be valid for message %d", i+1, j+1)
				}
			}
		}
	}
}

// TestEmptyAndZeroLengthMessages verifies edge cases with message handling
func TestEmptyAndZeroLengthMessages(t *testing.T) {
	privKey, pubKey, err := GenerateKeyPair(standardLevels, standardLmType, standardOtsType)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}
	
	workingKey, err := LoadWorkingKey(privKey, standardLevels, standardLmType, standardOtsType, 0)
	if err != nil {
		t.Fatalf("Failed to load working key: %v", err)
	}
	defer workingKey.Free()
	
	// Test empty message
	emptyMsg := []byte("")
	signature, err := workingKey.GenerateSignature(emptyMsg)
	if err != nil {
		// Empty messages might not be supported, which is acceptable
		return
	}
	
	valid, err := VerifySignature(pubKey, emptyMsg, signature)
	if err != nil {
		t.Fatalf("Failed to verify empty message signature: %v", err)
	}
	if !valid {
		t.Error("Signature for empty message should be valid")
	}
	
	// Test with wrong signature for empty message
	nonEmptyMsg := []byte("non-empty")
	nonEmptySig, err := workingKey.GenerateSignature(nonEmptyMsg)
	if err != nil {
		t.Fatalf("Failed to sign non-empty message: %v", err)
	}
	
	valid, err = VerifySignature(pubKey, emptyMsg, nonEmptySig)
	if err != nil {
		// Error is acceptable
		return
	}
	if valid {
		t.Error("Non-empty signature should not be valid for empty message")
	}
}
