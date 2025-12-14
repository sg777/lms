package client

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/verifiable-state-chains/lms/models"
)

// ExampleHSMClientUsage demonstrates how to use the HSM client library
// This is a reference implementation showing the complete workflow
func ExampleHSMClientUsage() {
	// Step 1: Create HSM client
	// Connect to any service node (will auto-forward to leader)
	serviceEndpoints := []string{
		"http://159.69.23.29:8080",
		"http://159.69.23.30:8080",
		"http://159.69.23.31:8080",
	}
	
	hsmID := "hsm-partition-001"
	client := NewHSMClient(serviceEndpoints, hsmID)
	
	// Step 2: Compute genesis hash (from LMS public key + system bundle)
	// In real implementation, this would come from HSM configuration
	lmsPublicKey := []byte("lms-public-key-from-hsm")
	systemBundle := []byte("system-bundle-config")
	genesisHash := ComputeGenesisHash(lmsPublicKey, systemBundle)
	
	// Step 3: Create protocol instance (without blockchain fallback)
	protocol := NewHSMProtocol(client, genesisHash, nil)
	
	// Step 4: Sync state (fetch latest head from service)
	fmt.Println("Syncing state with service...")
	if err := protocol.SyncState(); err != nil {
		log.Fatalf("Failed to sync state: %v", err)
	}
	
	state := protocol.GetState()
	fmt.Printf("Current state: LMS Index=%d, Sequence=%d\n",
		state.CurrentLMSIndex, state.SequenceNumber)
	
	// Step 5: Get next usable index
	nextIndex := protocol.GetNextUsableIndex()
	fmt.Printf("Next usable LMS index: %d\n", nextIndex)
	
	// Step 6: Hash the message to be signed
	message := []byte("application message to sign")
	messageHash := base64.StdEncoding.EncodeToString(message) // Simplified
	
	// Step 7: Create attestation payload
	timestamp := time.Now().UTC().Format(time.RFC3339)
	payload, err := protocol.CreateAttestationPayload(
		nextIndex,
		messageHash,
		timestamp,
		"example-metadata",
	)
	if err != nil {
		log.Fatalf("Failed to create payload: %v", err)
	}
	
	fmt.Printf("Created payload: LMS Index=%d, Sequence=%d\n",
		payload.LMSIndex, payload.SequenceNumber)
	
	// Step 8: HSM signs the payload (in real implementation, this happens in HSM)
	// For this example, we'll use mock signatures
	signature := generateMockSignature(payload)
	certificate := generateMockCertificate()
	
	// Step 9: Create attestation response
	attestation, err := protocol.CreateAttestationResponse(
		payload,
		"LMS_ATTEST_POLICY",
		"PS256",
		signature,
		certificate,
	)
	if err != nil {
		log.Fatalf("Failed to create attestation: %v", err)
	}
	
	// Step 10: Commit attestation (with timeout)
	timeout := 5 * time.Second
	fmt.Println("Committing attestation to service...")
	committed, raftIndex, raftTerm, err := protocol.CommitAttestation(attestation, timeout)
	
	if err != nil || !committed {
		// Discard Rule: If rejected, the index is automatically marked as unusable
		log.Fatalf("Attestation rejected - index %d is now unusable: %v", nextIndex, err)
	}
	
	fmt.Printf("✅ Attestation committed! Raft Index=%d, Term=%d\n", raftIndex, raftTerm)
	
	// Step 11: Verify state was updated
	updatedState := protocol.GetState()
	fmt.Printf("Updated state: LMS Index=%d, Sequence=%d\n",
		updatedState.CurrentLMSIndex, updatedState.SequenceNumber)
}

// ExampleCompleteWorkflow demonstrates using the convenience method
func ExampleCompleteWorkflow() {
	serviceEndpoints := []string{
		"http://159.69.23.29:8080",
		"http://159.69.23.30:8080",
		"http://159.69.23.31:8080",
	}
	
	client := NewHSMClient(serviceEndpoints, "hsm-partition-001")
	genesisHash := ComputeGenesisHash([]byte("lms-key"), []byte("system-bundle"))
	protocol := NewHSMProtocol(client, genesisHash, nil)
	
	// Hash the message
	message := []byte("message to sign")
	messageHash := base64.StdEncoding.EncodeToString(message)
	
	// Use complete workflow (all steps combined)
	committed, raftIndex, raftTerm, err := protocol.CompleteWorkflow(
		messageHash,
		"LMS_ATTEST_POLICY",
		"PS256",
		generateMockSignature(nil),
		generateMockCertificate(),
		5*time.Second,
	)
	
	if err != nil || !committed {
		log.Fatalf("Workflow failed: %v", err)
	}
	
	fmt.Printf("✅ Complete workflow succeeded! Raft Index=%d, Term=%d\n",
		raftIndex, raftTerm)
}

// Helper functions for examples (mock implementations)

func generateMockSignature(payload *models.ChainedPayload) string {
	// In real implementation, HSM would sign the payload
	// This is just a mock
	sig := make([]byte, 64)
	rand.Read(sig)
	return base64.StdEncoding.EncodeToString(sig)
}

func generateMockCertificate() string {
	// In real implementation, HSM would generate certificate
	// This is just a mock PEM
	cert := []byte("-----BEGIN CERTIFICATE-----\nMOCK_CERTIFICATE\n-----END CERTIFICATE-----")
	return base64.StdEncoding.EncodeToString(cert)
}

