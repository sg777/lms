package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestChainedPayloadSerialization(t *testing.T) {
	payload := &ChainedPayload{
		PreviousHash:   "test_hash_123",
		LMSIndex:       42,
		MessageSigned:  "message_hash_456",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Metadata:       "test",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded ChainedPayload
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.PreviousHash != payload.PreviousHash {
		t.Errorf("PreviousHash mismatch: got %s, want %s", decoded.PreviousHash, payload.PreviousHash)
	}
	if decoded.LMSIndex != payload.LMSIndex {
		t.Errorf("LMSIndex mismatch: got %d, want %d", decoded.LMSIndex, payload.LMSIndex)
	}
}

func TestAttestationResponseChainedPayload(t *testing.T) {
	ar := &AttestationResponse{}
	ar.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	ar.AttestationResponse.Policy.Algorithm = "PS256"

	payload := &ChainedPayload{
		PreviousHash:   "hash123",
		LMSIndex:       1,
		MessageSigned:  "msg456",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	if err := ar.SetChainedPayload(payload); err != nil {
		t.Fatalf("Failed to set chained payload: %v", err)
	}

	decoded, err := ar.GetChainedPayload()
	if err != nil {
		t.Fatalf("Failed to get chained payload: %v", err)
	}

	if decoded.PreviousHash != payload.PreviousHash {
		t.Errorf("PreviousHash mismatch: got %s, want %s", decoded.PreviousHash, payload.PreviousHash)
	}
	if decoded.LMSIndex != payload.LMSIndex {
		t.Errorf("LMSIndex mismatch: got %d, want %d", decoded.LMSIndex, payload.LMSIndex)
	}
}

func TestAttestationResponseHash(t *testing.T) {
	ar1 := &AttestationResponse{}
	ar1.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"

	ar2 := &AttestationResponse{}
	ar2.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"

	hash1, err := ar1.ComputeHash()
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	hash2, err := ar2.ComputeHash()
	if err != nil {
		t.Fatalf("Failed to compute hash: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hashes should be equal for identical attestations")
	}

	// Modify one
	ar2.AttestationResponse.Policy.Value = "DIFFERENT"
	hash2, _ = ar2.ComputeHash()
	if hash1 == hash2 {
		t.Errorf("Hashes should differ for different attestations")
	}
}

func TestLogEntrySerialization(t *testing.T) {
	ar := &AttestationResponse{}
	ar.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"

	payload := &ChainedPayload{
		PreviousHash:   "prev_hash",
		LMSIndex:       5,
		MessageSigned:  "msg",
		SequenceNumber: 10,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	ar.SetChainedPayload(payload)

	entry := &LogEntry{
		Index:       100,
		Term:        5,
		Timestamp:    time.Now(),
		Attestation: ar,
		CommittedBy: "hsm1",
	}

	data, err := entry.ToBytes()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	var decoded LogEntry
	if err := decoded.FromBytes(data); err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if decoded.Index != entry.Index {
		t.Errorf("Index mismatch: got %d, want %d", decoded.Index, entry.Index)
	}

	lmsIndex, err := decoded.GetLMSIndex()
	if err != nil {
		t.Fatalf("Failed to get LMS index: %v", err)
	}
	if lmsIndex != 5 {
		t.Errorf("LMSIndex mismatch: got %d, want 5", lmsIndex)
	}
}

func TestCreateGenesisPayload(t *testing.T) {
	genesis := CreateGenesisPayload("lms_pub_key_hash", 0, "message_hash")
	
	if genesis.PreviousHash != "lms_pub_key_hash" {
		t.Errorf("Genesis PreviousHash should be LMS public key hash")
	}
	if genesis.LMSIndex != 0 {
		t.Errorf("Genesis LMSIndex should be 0")
	}
	if genesis.SequenceNumber != 0 {
		t.Errorf("Genesis SequenceNumber should be 0")
	}
}

