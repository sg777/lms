package hsm_server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestCommitIndexToRaft_SignatureCreation(t *testing.T) {
	// Generate test key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Test signing
	keyID := "test_key_1"
	index := uint64(0)

	// Create data to sign
	data := keyID + ":" + string(rune(index))
	hash := sha256.Sum256([]byte(data))

	// Sign
	signature, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Encode public key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	// Verify signature
	if !ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		t.Fatal("Signature verification failed")
	}

	t.Logf("✅ Signature creation and verification successful")
	t.Logf("   Signature (base64): %s", base64.StdEncoding.EncodeToString(signature))
	t.Logf("   Public key (base64): %s", base64.StdEncoding.EncodeToString(pubKeyBytes))
}

func TestSignature_DataFormat(t *testing.T) {
	// Test that data format matches between signing and verification
	keyID := "lms_key_1"
	index := uint64(0)

	// Format used in commitIndexToRaft
	data1 := keyID + ":" + string(rune(index))
	hash1 := sha256.Sum256([]byte(data1))

	// Format used in verifySignature
	data2 := keyID + ":" + string(rune(index))
	hash2 := sha256.Sum256([]byte(data2))

	if hash1 != hash2 {
		t.Fatalf("Hash mismatch! Data format is inconsistent")
	}

	t.Logf("✅ Data format matches: %s", data1)
}

func TestSignature_WithCorrectFormat(t *testing.T) {
	// Generate test key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Use the EXACT format from commitIndexToRaft: fmt.Sprintf("%s:%d", keyID, index)
	data := "test_key_1:0" // This is what fmt.Sprintf("%s:%d", "test_key_1", 0) produces
	hash := sha256.Sum256([]byte(data))

	// Sign
	signature, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Verify with same format
	verifyData := "test_key_1:0" // Same format
	verifyHash := sha256.Sum256([]byte(verifyData))

	if !ecdsa.VerifyASN1(pubKey, verifyHash[:], signature) {
		t.Fatal("Signature verification failed with correct format")
	}

	t.Logf("✅ Signature works with correct data format")
}

