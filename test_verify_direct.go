package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	// Exact values from the logs
	signatureBase64 := "MEUCIQD1B/B0bOsw1RbPO1pm3LIAvNaagbLgiWvRtkZvH1386AIgbsjABD2HRojn+YZxMnO/mBYGbNGXzPPokKpGIRuSlLI="
	publicKeyBase64 := "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEQF9A7vbZnizzStr4tIc+RDGIsEM5Fm6VVQ05q8ZjUUV97D83rsOYG0pwoaIQliiB5M8miMXPdWYb0sPJKjCNKg=="
	data := "lms_key_1:0"
	
	fmt.Printf("Data: %s\n", data)
	hash := sha256.Sum256([]byte(data))
	fmt.Printf("Hash: %x\n", hash)
	
	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		fmt.Printf("❌ Failed to decode signature: %v\n", err)
		return
	}
	fmt.Printf("Signature length: %d bytes\n", len(sigBytes))
	
	// Decode public key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		fmt.Printf("❌ Failed to decode public key: %v\n", err)
		return
	}
	fmt.Printf("Public key length: %d bytes\n", len(pubKeyBytes))
	
	// Parse public key
	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		fmt.Printf("❌ Failed to parse public key: %v\n", err)
		return
	}
	
	pubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		fmt.Printf("❌ Not an ECDSA public key\n")
		return
	}
	
	fmt.Printf("Public key curve: %s\n", pubKey.Curve.Params().Name)
	
	// Verify
	if ecdsa.VerifyASN1(pubKey, hash[:], sigBytes) {
		fmt.Println("✅ Verification SUCCESS")
	} else {
		fmt.Println("❌ Verification FAILED")
		
		// Try loading the actual key from file and verify
		fmt.Println("\nTrying with key from file...")
		privKeyData, err := os.ReadFile("keys/attestation_private_key.pem")
		if err != nil {
			fmt.Printf("Cannot read private key: %v\n", err)
			return
		}
		
		privBlock, _ := pem.Decode(privKeyData)
		privKey, err := x509.ParseECPrivateKey(privBlock.Bytes)
		if err != nil {
			fmt.Printf("Cannot parse private key: %v\n", err)
			return
		}
		
		filePubKey := &privKey.PublicKey
		fmt.Printf("File public key curve: %s\n", filePubKey.Curve.Params().Name)
		
		// Check if keys match
		if pubKey.X.Cmp(filePubKey.X) == 0 && pubKey.Y.Cmp(filePubKey.Y) == 0 {
			fmt.Println("Keys match!")
		} else {
			fmt.Println("❌ Keys DON'T match!")
			fmt.Printf("Received X: %x\n", pubKey.X.Bytes())
			fmt.Printf("File X:     %x\n", filePubKey.X.Bytes())
		}
	}
}

