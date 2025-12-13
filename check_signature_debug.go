package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	// Load keys
	privKeyData, _ := os.ReadFile("keys/attestation_private_key.pem")
	privBlock, _ := pem.Decode(privKeyData)
	privKey, _ := x509.ParseECPrivateKey(privBlock.Bytes)

	pubKeyData, _ := os.ReadFile("keys/attestation_public_key.pem")
	pubBlock, _ := pem.Decode(pubKeyData)
	pubKeyInterface, _ := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	pubKey := pubKeyInterface.(*ecdsa.PublicKey)

	// Test data
	keyID := "lms_key_1"
	index := uint64(0)
	data := fmt.Sprintf("%s:%d", keyID, index)
	hash := sha256.Sum256([]byte(data))

	fmt.Printf("Data: %s\n", data)
	fmt.Printf("Hash: %x\n", hash)

	// Sign
	signature, _ := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	sigBase64 := base64.StdEncoding.EncodeToString(signature)

	// Marshal public key
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(pubKey)
	pubKeyBase64 := base64.StdEncoding.EncodeToString(pubKeyBytes)

	fmt.Printf("Signature (base64): %s\n", sigBase64)
	fmt.Printf("Public key (base64): %s\n", pubKeyBase64)

	// Verify
	if ecdsa.VerifyASN1(pubKey, hash[:], signature) {
		fmt.Println("✅ Verification SUCCESS")
	} else {
		fmt.Println("❌ Verification FAILED")
	}

	// Test decoding
	decodedSig, _ := base64.StdEncoding.DecodeString(sigBase64)
	decodedPubKeyBytes, _ := base64.StdEncoding.DecodeString(pubKeyBase64)
	decodedPubKeyInterface, _ := x509.ParsePKIXPublicKey(decodedPubKeyBytes)
	decodedPubKey := decodedPubKeyInterface.(*ecdsa.PublicKey)

	if ecdsa.VerifyASN1(decodedPubKey, hash[:], decodedSig) {
		fmt.Println("✅ Verification with decoded data SUCCESS")
	} else {
		fmt.Println("❌ Verification with decoded data FAILED")
	}
}

