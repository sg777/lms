package tests

import (
	"testing"
	"time"

	"github.com/verifiable-state-chains/lms/client"
	"github.com/verifiable-state-chains/lms/models"
	"github.com/verifiable-state-chains/lms/simulator"
)

// TestHSMClientBasic tests basic HSM client functionality
// This test doesn't require a running service
func TestHSMClientBasic(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	client := client.NewHSMClient(endpoints, "test-hsm-1")
	
	if client == nil {
		t.Fatal("NewHSMClient returned nil")
	}
}

// TestHSMProtocolBasic tests basic HSM protocol functionality
func TestHSMProtocolBasic(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	hsmClient := client.NewHSMClient(endpoints, genesisHash)
	protocol := client.NewHSMProtocol(hsmClient, genesisHash, nil)
	
	state := protocol.GetState()
	if state == nil {
		t.Fatal("GetState returned nil")
	}
	
	if state.CurrentLMSIndex != 0 {
		t.Errorf("Expected initial LMS index 0, got %d", state.CurrentLMSIndex)
	}
	
	nextIndex := protocol.GetNextUsableIndex()
	if nextIndex != 1 {
		t.Errorf("Expected next usable index 1, got %d", nextIndex)
	}
}

// TestHSMProtocolCreatePayload tests payload creation
func TestHSMProtocolCreatePayload(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	hsmClient := client.NewHSMClient(endpoints, genesisHash)
	protocol := client.NewHSMProtocol(hsmClient, genesisHash, nil)
	
	// Create genesis payload
	payload, err := protocol.CreateAttestationPayload(
		0,
		"test-message-hash",
		time.Now().UTC().Format(time.RFC3339),
		"test-metadata",
	)
	
	if err != nil {
		t.Fatalf("Failed to create payload: %v", err)
	}
	
	if payload.PreviousHash != genesisHash {
		t.Errorf("Expected previous_hash to be genesis hash, got %s", payload.PreviousHash)
	}
	
	if payload.LMSIndex != 0 {
		t.Errorf("Expected LMS index 0, got %d", payload.LMSIndex)
	}
	
	if payload.SequenceNumber != 1 {
		t.Errorf("Expected sequence number 1, got %d", payload.SequenceNumber)
	}
}

// TestHSMProtocolCreateAttestationResponse tests attestation response creation
func TestHSMProtocolCreateAttestationResponse(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	hsmClient := client.NewHSMClient(endpoints, genesisHash)
	protocol := client.NewHSMProtocol(hsmClient, genesisHash, nil)
	
	payload := &models.ChainedPayload{
		PreviousHash:   genesisHash,
		LMSIndex:       0,
		MessageSigned:  "test-message",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	
	attestation, err := protocol.CreateAttestationResponse(
		payload,
		"LMS_ATTEST_POLICY",
		"PS256",
		"mock-signature",
		"mock-certificate",
	)
	
	if err != nil {
		t.Fatalf("Failed to create attestation response: %v", err)
	}
	
	if attestation.AttestationResponse.Policy.Value != "LMS_ATTEST_POLICY" {
		t.Errorf("Expected policy 'LMS_ATTEST_POLICY', got '%s'",
			attestation.AttestationResponse.Policy.Value)
	}
	
	// Verify payload is embedded
	decodedPayload, err := attestation.GetChainedPayload()
	if err != nil {
		t.Fatalf("Failed to get chained payload: %v", err)
	}
	
	if decodedPayload.LMSIndex != 0 {
		t.Errorf("Expected LMS index 0, got %d", decodedPayload.LMSIndex)
	}
}

// TestHSMSimulatorBasic tests the HSM simulator
func TestHSMSimulatorBasic(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	sim := simulator.NewHSMSimulator("test-hsm-1", endpoints, genesisHash)
	
	if sim.GetID() != "test-hsm-1" {
		t.Errorf("Expected HSM ID 'test-hsm-1', got '%s'", sim.GetID())
	}
	
	stats := sim.GetStats()
	if stats.TotalAttestations != 0 {
		t.Errorf("Expected 0 attestations initially, got %d", stats.TotalAttestations)
	}
	
	attestations := sim.GetAttestations()
	if len(attestations) != 0 {
		t.Errorf("Expected 0 attestations, got %d", len(attestations))
	}
	
	errors := sim.GetErrors()
	if len(errors) != 0 {
		t.Errorf("Expected 0 errors initially, got %d", len(errors))
	}
}

// TestHSMSimulatorPoolBasic tests the HSM simulator pool
func TestHSMSimulatorPoolBasic(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	pool := simulator.NewHSMSimulatorPool(endpoints, genesisHash, 5)
	
	if pool.GetCount() != 5 {
		t.Errorf("Expected 5 simulators, got %d", pool.GetCount())
	}
	
	sim := pool.GetSimulator(0)
	if sim == nil {
		t.Fatal("Expected simulator at index 0, got nil")
	}
	
	if sim.GetID() != "hsm-sim-1" {
		t.Errorf("Expected HSM ID 'hsm-sim-1', got '%s'", sim.GetID())
	}
}

