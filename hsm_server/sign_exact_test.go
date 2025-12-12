package hsm_server

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/fsm"
)

// TestExactCommandFlow tests the EXACT flow from the command:
// ./hsm-client sign -key-id lms_key_1 -msg "hello"
func TestExactCommandFlow(t *testing.T) {
	// Load the actual keys from the keys directory (same as production)
	privKey, pubKey, err := LoadAttestationKeyPair()
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}

	// Create FSM (same as production)
	keyIndexFSM, err := fsm.NewKeyIndexFSM("./keys/attestation_public_key.pem")
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}

	// EXACT data from the command: key_id = "lms_key_1", index = 0
	keyID := "lms_key_1"
	index := uint64(0)

	// Step 1: Sign (EXACT same code as commitIndexToRaft)
	data := "lms_key_1:0" // fmt.Sprintf("%s:%d", keyID, index)
	hash := sha256.Sum256([]byte(data))
	
	t.Logf("Signing data: %s", data)
	t.Logf("Hash: %x", hash)
	
	signature, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	if err != nil {
		t.Fatalf("Failed to sign: %v", err)
	}

	// Step 2: Marshal public key (EXACT same code)
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	// Step 3: Create entry (EXACT same format as sent to Raft)
	entry := fsm.KeyIndexEntry{
		KeyID:     keyID,
		Index:     index,
		Signature: base64.StdEncoding.EncodeToString(signature),
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
	}

	t.Logf("Entry: key_id=%s, index=%d", entry.KeyID, entry.Index)
	t.Logf("Signature (base64): %s", entry.Signature)
	t.Logf("Public key (base64): %s", entry.PublicKey)

	// Step 4: Serialize (EXACT same as sent to Raft)
	entryData, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("Failed to marshal entry: %v", err)
	}

	// Step 5: Apply to FSM (EXACT same as Raft Apply)
	log := &raft.Log{
		Type:  raft.LogCommand,
		Index: 1,
		Term:  1,
		Data:  entryData,
	}

	result := keyIndexFSM.Apply(log)
	resultStr, ok := result.(string)
	if !ok {
		t.Fatalf("Expected string result, got: %T", result)
	}

	if resultStr[:5] == "Error" {
		t.Fatalf("❌ FAILED: %s", resultStr)
	}

	// Step 6: Verify it was stored
	storedIndex, exists := keyIndexFSM.GetKeyIndex(keyID)
	if !exists {
		t.Fatal("❌ Index was not stored")
	}
	if storedIndex != index {
		t.Fatalf("❌ Stored index mismatch: expected %d, got %d", index, storedIndex)
	}

	t.Logf("✅ SUCCESS: Exact command flow works!")
	t.Logf("   Stored: key_id=%s, index=%d", keyID, storedIndex)
}

// TestExactCommandFlow_VerifySignatureOnly tests just the signature verification
// with the exact data from the command
func TestExactCommandFlow_VerifySignatureOnly(t *testing.T) {
	// Load actual keys
	privKey, pubKey, err := LoadAttestationKeyPair()
	if err != nil {
		t.Fatalf("Failed to load keys: %v", err)
	}

	// Create FSM
	keyIndexFSM, err := fsm.NewKeyIndexFSM("./keys/attestation_public_key.pem")
	if err != nil {
		t.Fatalf("Failed to create FSM: %v", err)
	}

	// EXACT data: key_id = "lms_key_1", index = 0
	keyID := "lms_key_1"
	index := uint64(0)

	// Sign (EXACT same as commitIndexToRaft)
	data := "lms_key_1:0"
	hash := sha256.Sum256([]byte(data))
	signature, _ := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(pubKey)

	// Create entry
	entry := fsm.KeyIndexEntry{
		KeyID:     keyID,
		Index:     index,
		Signature: base64.StdEncoding.EncodeToString(signature),
		PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes),
	}

	// Verify using the FSM's VerifySignature method
	err = keyIndexFSM.VerifySignature(&entry)
	if err != nil {
		t.Fatalf("❌ Signature verification failed: %v", err)
	}

	t.Logf("✅ Signature verification successful with exact command data")
}

