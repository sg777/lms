package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	// Create keys directory
	os.MkdirAll("./keys", 0700)

	// Generate fresh key pair
	fmt.Println("Generating fresh EC attestation key pair...")
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		fmt.Printf("Failed to generate key: %v\n", err)
		os.Exit(1)
	}
	pubKey := &privKey.PublicKey

	// Save private key
	privKeyBytes, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		fmt.Printf("Failed to marshal private key: %v\n", err)
		os.Exit(1)
	}

	privBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privKeyBytes,
	}

	privFile, err := os.OpenFile("./keys/attestation_private_key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Failed to create private key file: %v\n", err)
		os.Exit(1)
	}
	pem.Encode(privFile, privBlock)
	privFile.Close()

	// Save public key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		fmt.Printf("Failed to marshal public key: %v\n", err)
		os.Exit(1)
	}

	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	}

	pubFile, err := os.OpenFile("./keys/attestation_public_key.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Failed to create public key file: %v\n", err)
		os.Exit(1)
	}
	pem.Encode(pubFile, pubBlock)
	pubFile.Close()

	// Verify they match
	privKeyData, _ := os.ReadFile("./keys/attestation_private_key.pem")
	privBlock2, _ := pem.Decode(privKeyData)
	privKey2, _ := x509.ParseECPrivateKey(privBlock2.Bytes)

	pubKeyData, _ := os.ReadFile("./keys/attestation_public_key.pem")
	pubBlock2, _ := pem.Decode(pubKeyData)
	pubKeyInterface, _ := x509.ParsePKIXPublicKey(pubBlock2.Bytes)
	pubKey2 := pubKeyInterface.(*ecdsa.PublicKey)

	// Check if they match
	if privKey2.PublicKey.X.Cmp(pubKey2.X) == 0 && privKey2.PublicKey.Y.Cmp(pubKey2.Y) == 0 {
		fmt.Println("✅ Keys are pairwise consistent!")
		fmt.Printf("   Public key X: %x\n", pubKey2.X.Bytes())
		fmt.Printf("   Public key Y: %x\n", pubKey2.Y.Bytes())
	} else {
		fmt.Println("❌ Keys DON'T match!")
		os.Exit(1)
	}

	fmt.Println("✅ Fresh key pair generated and verified!")
}

