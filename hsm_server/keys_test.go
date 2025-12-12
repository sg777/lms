package hsm_server

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

// TestLoadAttestationKeyPair_Consistency verifies that the keys loaded from files are pairwise consistent
func TestLoadAttestationKeyPair_Consistency(t *testing.T) {
	// Load keys from actual files
	privKey, pubKey, err := LoadAttestationKeyPair()
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}

	// Verify that the private key's public key matches the loaded public key
	privKeyPubKey := &privKey.PublicKey

	if !privKeyPubKey.Equal(pubKey) {
		t.Fatal("❌ Keys are NOT pairwise consistent!")
	}

	// Verify X and Y coordinates match
	if privKeyPubKey.X.Cmp(pubKey.X) != 0 || privKeyPubKey.Y.Cmp(pubKey.Y) != 0 {
		t.Fatalf("❌ Key coordinates don't match!\nPrivate key pub X: %x\nLoaded pub X:     %x\nPrivate key pub Y: %x\nLoaded pub Y:     %x",
			privKeyPubKey.X.Bytes(), pubKey.X.Bytes(), privKeyPubKey.Y.Bytes(), pubKey.Y.Bytes())
	}

	t.Logf("✅ Keys are pairwise consistent!")
	t.Logf("   Public key X: %x", pubKey.X.Bytes())
	t.Logf("   Public key Y: %x", pubKey.Y.Bytes())
}

// TestLoadAttestationKeyPair_LoadsFromFiles verifies keys are loaded from the correct files
func TestLoadAttestationKeyPair_LoadsFromFiles(t *testing.T) {
	privKeyPath := filepath.Join("./keys", "attestation_private_key.pem")
	pubKeyPath := filepath.Join("./keys", "attestation_public_key.pem")

	// Check files exist
	if _, err := os.Stat(privKeyPath); err != nil {
		t.Fatalf("Private key file not found: %s", privKeyPath)
	}

	if _, err := os.Stat(pubKeyPath); err != nil {
		t.Fatalf("Public key file not found: %s", pubKeyPath)
	}

	// Load keys
	privKey, pubKey, err := LoadAttestationKeyPair()
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}

	// Load keys directly from files to verify
	privKeyData, err := os.ReadFile(privKeyPath)
	if err != nil {
		t.Fatalf("Failed to read private key file: %v", err)
	}

	privBlock, _ := pem.Decode(privKeyData)
	if privBlock == nil {
		t.Fatal("Failed to decode private key PEM")
	}

	filePrivKey, err := x509.ParseECPrivateKey(privBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		t.Fatalf("Failed to read public key file: %v", err)
	}

	pubBlock, _ := pem.Decode(pubKeyData)
	if pubBlock == nil {
		t.Fatal("Failed to decode public key PEM")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse public key: %v", err)
	}

	filePubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		t.Fatal("Not an ECDSA public key")
	}

	// Verify loaded keys match file keys
	if !privKey.Equal(filePrivKey) {
		t.Fatal("❌ Loaded private key doesn't match file private key")
	}

	if !pubKey.Equal(filePubKey) {
		t.Fatal("❌ Loaded public key doesn't match file public key")
	}

	// CRITICAL: Verify pairwise consistency between private and public keys
	privKeyDerivedPubKey := &filePrivKey.PublicKey
	if !privKeyDerivedPubKey.Equal(filePubKey) {
		t.Fatal("❌ Keys from files are NOT pairwise consistent! Private key's public key doesn't match public key file!")
	}

	t.Logf("✅ Keys loaded correctly and are pairwise consistent!")
}

