package fsm

import (
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/models"
)

func TestHashChainFSM_Genesis(t *testing.T) {
	genesisHash := "genesis_hash_123"
	fsm := NewHashChainFSM(genesisHash)

	// Create genesis attestation
	genesisPayload := models.CreateGenesisPayload(genesisHash, 0, "message_hash_0")
	genesisAttestation := &models.AttestationResponse{}
	genesisAttestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	genesisAttestation.AttestationResponse.Policy.Algorithm = "PS256"
	genesisAttestation.SetChainedPayload(genesisPayload)

	// Serialize for Raft log
	data, err := genesisAttestation.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Apply as Raft log entry
	log := &raft.Log{
		Type:  raft.LogCommand,
		Index: 1,
		Term:  1,
		Data:  data,
	}

	result := fsm.Apply(log)
	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Verify attestation was stored
	latest, err := fsm.GetLatestAttestation()
	if err != nil {
		t.Fatalf("Failed to get latest attestation: %v", err)
	}

	if latest == nil {
		t.Fatal("Expected non-nil attestation")
	}

	// Verify chain integrity
	if err := fsm.VerifyChainIntegrity(); err != nil {
		t.Errorf("Chain integrity check failed: %v", err)
	}
}

func TestHashChainFSM_ChainLinking(t *testing.T) {
	genesisHash := "genesis_hash_123"
	fsm := NewHashChainFSM(genesisHash)

	// Create and apply genesis
	genesisPayload := models.CreateGenesisPayload(genesisHash, 0, "message_hash_0")
	genesisAttestation := &models.AttestationResponse{}
	genesisAttestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	genesisAttestation.SetChainedPayload(genesisPayload)

	genesisData, _ := genesisAttestation.ToJSON()
	fsm.Apply(&raft.Log{Type: raft.LogCommand, Index: 1, Term: 1, Data: genesisData})

	// Get hash of genesis for next entry
	genesisHashComputed, err := fsm.GetChainHeadHash()
	if err != nil {
		t.Fatalf("Failed to get chain head hash: %v", err)
	}

	// Create second attestation
	payload2 := &models.ChainedPayload{
		PreviousHash:   genesisHashComputed,
		LMSIndex:       1,
		MessageSigned:  "message_hash_1",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	attestation2 := &models.AttestationResponse{}
	attestation2.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	attestation2.SetChainedPayload(payload2)

	data2, _ := attestation2.ToJSON()
	result := fsm.Apply(&raft.Log{Type: raft.LogCommand, Index: 2, Term: 1, Data: data2})

	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Verify chain integrity
	if err := fsm.VerifyChainIntegrity(); err != nil {
		t.Errorf("Chain integrity check failed: %v", err)
	}

	// Verify we can get both entries
	count := fsm.GetLogCount()
	if count != 2 {
		t.Errorf("Expected 2 log entries, got %d", count)
	}
}

func TestHashChainFSM_InvalidHashChain(t *testing.T) {
	genesisHash := "genesis_hash_123"
	fsm := NewHashChainFSM(genesisHash)

	// Create and apply genesis
	genesisPayload := models.CreateGenesisPayload(genesisHash, 0, "message_hash_0")
	genesisAttestation := &models.AttestationResponse{}
	genesisAttestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	genesisAttestation.SetChainedPayload(genesisPayload)

	genesisData, _ := genesisAttestation.ToJSON()
	fsm.Apply(&raft.Log{Type: raft.LogCommand, Index: 1, Term: 1, Data: genesisData})

	// Create second attestation with WRONG previous_hash
	payload2 := &models.ChainedPayload{
		PreviousHash:   "wrong_hash", // This should fail
		LMSIndex:       1,
		MessageSigned:  "message_hash_1",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	attestation2 := &models.AttestationResponse{}
	attestation2.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	attestation2.SetChainedPayload(payload2)

	data2, _ := attestation2.ToJSON()
	result := fsm.Apply(&raft.Log{Type: raft.LogCommand, Index: 2, Term: 1, Data: data2})

	// Result should contain an error
	if result == nil {
		t.Fatal("Expected error result")
	}

	resultStr := result.(string)
	if resultStr[:5] != "Error" {
		t.Errorf("Expected error result, got: %s", resultStr)
	}

	// Verify only genesis entry exists
	count := fsm.GetLogCount()
	if count != 1 {
		t.Errorf("Expected 1 log entry (genesis only), got %d", count)
	}
}

func TestHashChainFSM_Monotonicity(t *testing.T) {
	genesisHash := "genesis_hash_123"
	fsm := NewHashChainFSM(genesisHash)

	// Create and apply genesis
	genesisPayload := models.CreateGenesisPayload(genesisHash, 0, "message_hash_0")
	genesisAttestation := &models.AttestationResponse{}
	genesisAttestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	genesisAttestation.SetChainedPayload(genesisPayload)

	genesisData, _ := genesisAttestation.ToJSON()
	fsm.Apply(&raft.Log{Type: raft.LogCommand, Index: 1, Term: 1, Data: genesisData})

	genesisHashComputed, _ := fsm.GetChainHeadHash()

	// Try to create entry with non-monotonic sequence number
	payload2 := &models.ChainedPayload{
		PreviousHash:   genesisHashComputed,
		LMSIndex:       1,
		MessageSigned:  "message_hash_1",
		SequenceNumber: 0, // Same as genesis (should fail)
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	attestation2 := &models.AttestationResponse{}
	attestation2.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	attestation2.SetChainedPayload(payload2)

	data2, _ := attestation2.ToJSON()
	result := fsm.Apply(&raft.Log{Type: raft.LogCommand, Index: 2, Term: 1, Data: data2})

	// Should fail due to non-monotonic sequence
	if result == nil {
		t.Fatal("Expected error result")
	}

	resultStr := result.(string)
	if resultStr[:5] != "Error" {
		t.Errorf("Expected error for non-monotonic sequence, got: %s", resultStr)
	}
}

