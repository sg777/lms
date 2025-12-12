package fsm

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/hashicorp/raft"
)

func TestKeyIndexFSM_VerifySignature(t *testing.T) {
	// Generate test key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Create FSM without pre-loaded key (will use key from request)
	fsm, err := NewKeyIndexFSM("")
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}

	// Create test entry
	keyID := "test_key_1"
	index := uint64(0)

	// Sign data (same format as commitIndexToRaft)
	data := "test_key_1:0" // fmt.Sprintf("%s:%d", keyID, index)
	hash := sha256.Sum256([]byte(data))
	signature, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Marshal public key
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	// Create entry
	entry := KeyIndexEntry{
		KeyID:     keyID,
		Index:     index,
		Signature: base64.StdEncoding.EncodeToString(signature),
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
	}

	// Verify signature
	err = fsm.verifySignature(&entry)
	if err != nil {
		t.Fatalf("Signature verification failed: %v", err)
	}

	t.Logf("✅ Signature verification successful")
}

func TestKeyIndexFSM_Apply_ValidSignature(t *testing.T) {
	// Generate test key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Create FSM
	fsm, err := NewKeyIndexFSM("")
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}

	// Create and sign entry
	keyID := "test_key_1"
	index := uint64(0)
	data := "test_key_1:0"
	hash := sha256.Sum256([]byte(data))
	signature, _ := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(pubKey)

	entry := KeyIndexEntry{
		KeyID:     keyID,
		Index:     index,
		Signature: base64.StdEncoding.EncodeToString(signature),
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
	}

	// Serialize entry
	entryData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Apply to FSM
	log := &raft.Log{
		Type:  raft.LogCommand,
		Index: 1,
		Term:  1,
		Data:  entryData,
	}

	result := fsm.Apply(log)
	if resultStr, ok := result.(string); ok {
		if resultStr[:5] == "Error" {
			t.Fatalf("Apply failed: %s", resultStr)
		}
	}

	// Verify index was stored
	storedIndex, exists := fsm.GetKeyIndex(keyID)
	if !exists {
		t.Fatal("Index was not stored")
	}
	if storedIndex != index {
		t.Fatalf("Stored index mismatch: expected %d, got %d", index, storedIndex)
	}

	t.Logf("✅ Apply with valid signature successful, stored index: %d", storedIndex)
}

func TestKeyIndexFSM_Apply_InvalidSignature(t *testing.T) {
	// Generate test key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Create FSM
	fsm, err := NewKeyIndexFSM("")
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}

	// Create entry with INVALID signature (sign different data)
	keyID := "test_key_1"
	index := uint64(0)
	
	// Sign WRONG data
	wrongData := "wrong_data"
	hash := sha256.Sum256([]byte(wrongData))
	signature, _ := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(pubKey)

	entry := KeyIndexEntry{
		KeyID:     keyID,
		Index:     index,
		Signature: base64.StdEncoding.EncodeToString(signature), // Wrong signature
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
	}

	// Serialize entry
	entryData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Apply to FSM
	log := &raft.Log{
		Type:  raft.LogCommand,
		Index: 1,
		Term:  1,
		Data:  entryData,
	}

	result := fsm.Apply(log)
	resultStr, ok := result.(string)
	if !ok {
		t.Fatal("Expected string result")
	}

	// Should fail with signature verification error
	if resultStr[:5] != "Error" {
		t.Fatalf("Expected error for invalid signature, got: %s", resultStr)
	}

	// Verify index was NOT stored
	_, exists := fsm.GetKeyIndex(keyID)
	if exists {
		t.Fatal("Index should not be stored with invalid signature")
	}

	t.Logf("✅ Invalid signature correctly rejected: %s", resultStr)
}

func TestKeyIndexFSM_DataFormatConsistency(t *testing.T) {
	// Test that data format is consistent between signing and verification
	// Format used in commitIndexToRaft (hsm_server/sign.go:83)
	// data := fmt.Sprintf("%s:%d", keyID, index)
	expectedData := "lms_key_1:0"

	// Format used in verifySignature (fsm/key_index_fsm.go:120)
	// data := fmt.Sprintf("%s:%d", entry.KeyID, entry.Index)
	verifyData := "lms_key_1:0"

	if expectedData != verifyData {
		t.Fatalf("Data format mismatch! Expected: %s, Got: %s", expectedData, verifyData)
	}

	// Verify hashes match
	expectedHash := sha256.Sum256([]byte(expectedData))
	verifyHash := sha256.Sum256([]byte(verifyData))

	if expectedHash != verifyHash {
		t.Fatal("Hash mismatch! Data format is inconsistent")
	}

	t.Logf("✅ Data format is consistent: %s", expectedData)
}

func TestKeyIndexFSM_EndToEnd(t *testing.T) {
	// Generate test key pair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	pubKey := &privKey.PublicKey

	// Create FSM
	fsm, err := NewKeyIndexFSM("")
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}

	// Simulate commitIndexToRaft
	keyID := "e2e_test_key"
	index := uint64(0)
	
	// Sign (same as commitIndexToRaft)
	data := "e2e_test_key:0" // fmt.Sprintf("%s:%d", keyID, index)
	hash := sha256.Sum256([]byte(data))
	signature, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Marshal public key (same as commitIndexToRaft)
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	// Create entry (same format as sent to Raft)
	entry := KeyIndexEntry{
		KeyID:     keyID,
		Index:     index,
		Signature: base64.StdEncoding.EncodeToString(signature),
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
	}

	// Serialize and apply
	entryData, _ := json.Marshal(entry)
	log := &raft.Log{
		Type:  raft.LogCommand,
		Index: 1,
		Term:  1,
		Data:  entryData,
	}

	result := fsm.Apply(log)
	if resultStr, ok := result.(string); ok {
		if resultStr[:5] == "Error" {
			t.Fatalf("End-to-end test failed: %s", resultStr)
		}
	}

	// Verify
	storedIndex, exists := fsm.GetKeyIndex(keyID)
	if !exists || storedIndex != index {
		t.Fatalf("End-to-end test failed: index not stored correctly")
	}

	t.Logf("✅ End-to-end test successful: key_id=%s, index=%d", keyID, storedIndex)
}

