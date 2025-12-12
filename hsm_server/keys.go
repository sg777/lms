package hsm_server

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	keyDir  = "./keys"
	privKeyFile = "attestation_private_key.pem"
	pubKeyFile  = "attestation_public_key.pem"
)

// LoadOrGenerateAttestationKeyPair loads or generates the EC attestation key pair
func LoadOrGenerateAttestationKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	// Create keys directory if it doesn't exist
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, nil, fmt.Errorf("failed to create keys directory: %v", err)
	}

	privKeyPath := filepath.Join(keyDir, privKeyFile)
	pubKeyPath := filepath.Join(keyDir, pubKeyFile)

	// Try to load existing keys
	privKey, pubKey, err := loadKeys(privKeyPath, pubKeyPath)
	if err == nil {
		fmt.Printf("✅ Loaded existing attestation keys from %s/\n", keyDir)
		return privKey, pubKey, nil
	}

	// Generate new keys (only if loading failed)
	fmt.Printf("⚠️  Failed to load existing keys: %v\n", err)
	fmt.Println("Generating new EC attestation key pair...")
	privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key: %v", err)
	}
	pubKey = &privKey.PublicKey

	// Save keys
	if err := savePrivateKey(privKey, privKeyPath); err != nil {
		return nil, nil, fmt.Errorf("failed to save private key: %v", err)
	}

	if err := savePublicKey(pubKey, pubKeyPath); err != nil {
		return nil, nil, fmt.Errorf("failed to save public key: %v", err)
	}

	fmt.Printf("✅ Attestation key pair generated and saved to %s/\n", keyDir)
	return privKey, pubKey, nil
}

func loadKeys(privKeyPath, pubKeyPath string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	// Load private key
	privKeyData, err := os.ReadFile(privKeyPath)
	if err != nil {
		return nil, nil, err
	}

	block, _ := pem.Decode(privKeyData)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode private key PEM")
	}

	privKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	// Load public key
	pubKeyData, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return nil, nil, err
	}

	block, _ = pem.Decode(pubKeyData)
	if block == nil {
		return nil, nil, fmt.Errorf("failed to decode public key PEM")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	pubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, fmt.Errorf("not an ECDSA public key")
	}

	return privKey, pubKey, nil
}

func savePrivateKey(privKey *ecdsa.PrivateKey, path string) error {
	keyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return err
	}

	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}

	return os.WriteFile(path, pem.EncodeToMemory(block), 0600)
}

func savePublicKey(pubKey *ecdsa.PublicKey, path string) error {
	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return err
	}

	block := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: keyBytes,
	}

	return os.WriteFile(path, pem.EncodeToMemory(block), 0644)
}

