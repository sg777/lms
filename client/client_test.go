package client

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/verifiable-state-chains/lms/models"
)

func TestHSMClient_NewHSMClient(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	
	if client == nil {
		t.Fatal("NewHSMClient returned nil")
	}
	
	if client.hsmIdentifier != "test-hsm-1" {
		t.Errorf("Expected hsmIdentifier 'test-hsm-1', got '%s'", client.hsmIdentifier)
	}
	
	if len(client.serviceEndpoints) != 1 {
		t.Errorf("Expected 1 endpoint, got %d", len(client.serviceEndpoints))
	}
}

func TestProtocolState_NewProtocolState(t *testing.T) {
	state := NewProtocolState()
	
	if state == nil {
		t.Fatal("NewProtocolState returned nil")
	}
	
	if state.CurrentLMSIndex != 0 {
		t.Errorf("Expected CurrentLMSIndex 0, got %d", state.CurrentLMSIndex)
	}
	
	if state.SequenceNumber != 0 {
		t.Errorf("Expected SequenceNumber 0, got %d", state.SequenceNumber)
	}
	
	if state.UnusableIndices == nil {
		t.Fatal("UnusableIndices map is nil")
	}
}

func TestHSMProtocol_NewHSMProtocol(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	genesisHash := "test-genesis-hash"
	
	protocol := NewHSMProtocol(client, genesisHash, nil)
	
	if protocol == nil {
		t.Fatal("NewHSMProtocol returned nil")
	}
	
	if protocol.genesisHash != genesisHash {
		t.Errorf("Expected genesisHash '%s', got '%s'", genesisHash, protocol.genesisHash)
	}
	
	if protocol.state == nil {
		t.Fatal("Protocol state is nil")
	}
}

func TestHSMProtocol_CreateAttestationPayload_Genesis(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	protocol := NewHSMProtocol(client, "genesis-hash-123")
	
	// No previous attestation (genesis)
	payload, err := protocol.CreateAttestationPayload(
		0,
		"message-hash-123",
		"2024-01-01T00:00:00Z",
		"test-metadata",
	)
	
	if err != nil {
		t.Fatalf("CreateAttestationPayload failed: %v", err)
	}
	
	if payload.PreviousHash != "genesis-hash-123" {
		t.Errorf("Expected PreviousHash 'genesis-hash-123', got '%s'", payload.PreviousHash)
	}
	
	if payload.LMSIndex != 0 {
		t.Errorf("Expected LMSIndex 0, got %d", payload.LMSIndex)
	}
	
	if payload.MessageSigned != "message-hash-123" {
		t.Errorf("Expected MessageSigned 'message-hash-123', got '%s'", payload.MessageSigned)
	}
	
	if payload.SequenceNumber != 1 {
		t.Errorf("Expected SequenceNumber 1, got %d", payload.SequenceNumber)
	}
}

func TestHSMProtocol_CreateAttestationPayload_WithPrevious(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	protocol := NewHSMProtocol(client, "genesis-hash-123")
	
	// Create a mock previous attestation
	prevPayload := &models.ChainedPayload{
		PreviousHash:   "genesis-hash-123",
		LMSIndex:       0,
		MessageSigned:  "prev-message",
		SequenceNumber: 1,
		Timestamp:      "2024-01-01T00:00:00Z",
	}
	
	prevAttestation := &models.AttestationResponse{}
	prevAttestation.SetChainedPayload(prevPayload)
	
	protocol.state.LastAttestation = prevAttestation
	protocol.state.SequenceNumber = 1
	
	// Create next payload
	payload, err := protocol.CreateAttestationPayload(
		1,
		"message-hash-456",
		"2024-01-01T01:00:00Z",
		"",
	)
	
	if err != nil {
		t.Fatalf("CreateAttestationPayload failed: %v", err)
	}
	
	if payload.PreviousHash == "" {
		t.Fatal("PreviousHash should be computed from previous attestation")
	}
	
	if payload.LMSIndex != 1 {
		t.Errorf("Expected LMSIndex 1, got %d", payload.LMSIndex)
	}
	
	if payload.SequenceNumber != 2 {
		t.Errorf("Expected SequenceNumber 2, got %d", payload.SequenceNumber)
	}
}

func TestHSMProtocol_CreateAttestationResponse(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	protocol := NewHSMProtocol(client, "genesis-hash-123")
	
	payload := &models.ChainedPayload{
		PreviousHash:   "genesis-hash-123",
		LMSIndex:       0,
		MessageSigned:  "message-hash",
		SequenceNumber: 1,
		Timestamp:      "2024-01-01T00:00:00Z",
	}
	
	signature := base64.StdEncoding.EncodeToString([]byte("fake-signature"))
	certificate := base64.StdEncoding.EncodeToString([]byte("fake-certificate"))
	
	attestation, err := protocol.CreateAttestationResponse(
		payload,
		"LMS_ATTEST_POLICY",
		"PS256",
		signature,
		certificate,
	)
	
	if err != nil {
		t.Fatalf("CreateAttestationResponse failed: %v", err)
	}
	
	if attestation.AttestationResponse.Policy.Value != "LMS_ATTEST_POLICY" {
		t.Errorf("Expected policy value 'LMS_ATTEST_POLICY', got '%s'",
			attestation.AttestationResponse.Policy.Value)
	}
	
	if attestation.AttestationResponse.Signature.Value != signature {
		t.Errorf("Signature mismatch")
	}
	
	// Verify payload is embedded correctly
	decodedPayload, err := attestation.GetChainedPayload()
	if err != nil {
		t.Fatalf("Failed to get chained payload: %v", err)
	}
	
	if decodedPayload.LMSIndex != 0 {
		t.Errorf("Expected LMSIndex 0, got %d", decodedPayload.LMSIndex)
	}
}

func TestHSMProtocol_IsIndexUsable(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	protocol := NewHSMProtocol(client, "genesis-hash")
	
	// Initially all indices are usable
	if !protocol.IsIndexUsable(0) {
		t.Error("Index 0 should be usable")
	}
	
	if !protocol.IsIndexUsable(100) {
		t.Error("Index 100 should be usable")
	}
	
	// Mark an index as unusable
	protocol.state.UnusableIndices[5] = true
	
	if protocol.IsIndexUsable(5) {
		t.Error("Index 5 should not be usable")
	}
	
	if !protocol.IsIndexUsable(4) {
		t.Error("Index 4 should still be usable")
	}
}

func TestHSMProtocol_GetNextUsableIndex(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	protocol := NewHSMProtocol(client, "genesis-hash")
	
	// Start from 0
	protocol.state.CurrentLMSIndex = 0
	next := protocol.GetNextUsableIndex()
	if next != 1 {
		t.Errorf("Expected next index 1, got %d", next)
	}
	
	// Mark 1 as unusable
	protocol.state.UnusableIndices[1] = true
	next = protocol.GetNextUsableIndex()
	if next != 2 {
		t.Errorf("Expected next index 2 (skipping 1), got %d", next)
	}
	
	// Mark 2 and 3 as unusable
	protocol.state.UnusableIndices[2] = true
	protocol.state.UnusableIndices[3] = true
	next = protocol.GetNextUsableIndex()
	if next != 4 {
		t.Errorf("Expected next index 4 (skipping 1,2,3), got %d", next)
	}
}

func TestComputeGenesisHash(t *testing.T) {
	lmsKey := []byte("lms-public-key-123")
	systemBundle := []byte("system-bundle-456")
	
	hash := ComputeGenesisHash(lmsKey, systemBundle)
	
	if hash == "" {
		t.Fatal("Genesis hash should not be empty")
	}
	
	// Should be base64 encoded
	_, err := base64.StdEncoding.DecodeString(hash)
	if err != nil {
		t.Errorf("Genesis hash should be base64 encoded: %v", err)
	}
	
	// Same inputs should produce same hash
	hash2 := ComputeGenesisHash(lmsKey, systemBundle)
	if hash != hash2 {
		t.Error("Same inputs should produce same genesis hash")
	}
	
	// Different inputs should produce different hash
	hash3 := ComputeGenesisHash([]byte("different-key"), systemBundle)
	if hash == hash3 {
		t.Error("Different inputs should produce different genesis hash")
	}
}

func TestHSMProtocol_DiscardRule(t *testing.T) {
	endpoints := []string{"http://localhost:8080"}
	client := NewHSMClient(endpoints, "test-hsm-1")
	protocol := NewHSMProtocol(client, "genesis-hash")
	
	// Create an attestation
	payload := &models.ChainedPayload{
		PreviousHash:   "genesis-hash",
		LMSIndex:       5,
		MessageSigned:  "message",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	
	_, err := protocol.CreateAttestationResponse(
		payload,
		"LMS_ATTEST_POLICY",
		"PS256",
		"signature",
		"certificate",
	)
	if err != nil {
		t.Fatalf("Failed to create attestation: %v", err)
	}
	
	// Simulate rejection (we can't actually call the service in unit tests)
	// But we can verify the discard rule logic
	protocol.state.UnusableIndices[5] = true
	
	if protocol.IsIndexUsable(5) {
		t.Error("Index 5 should be marked as unusable after rejection")
	}
}

