package hsm_server

import (
	"crypto/ecdsa"
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

// LoadAttestationKeyPair loads the EC attestation key pair from files
// Keys must be generated using OpenSSL: openssl ecparam -genkey -name prime256v1 -noout -out keys/attestation_private_key.pem
func LoadAttestationKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privKeyPath := filepath.Join(keyDir, privKeyFile)
	pubKeyPath := filepath.Join(keyDir, pubKeyFile)

	privKey, pubKey, err := loadKeys(privKeyPath, pubKeyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load attestation keys: %v (keys must be generated with OpenSSL)", err)
	}

	fmt.Printf("✅ Loaded attestation keys from %s/\n", keyDir)
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


