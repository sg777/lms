package client

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/verifiable-state-chains/lms/models"
)

// ProtocolState tracks the HSM's protocol state
type ProtocolState struct {
	CurrentLMSIndex   uint64 // Current LMS index being used
	SequenceNumber    uint64 // Monotonically increasing sequence number
	LastAttestation   *models.AttestationResponse // Last committed attestation
	LastRaftIndex     uint64 // Last Raft log index we've seen
	LastRaftTerm      uint64 // Last Raft term we've seen
	UnusableIndices   map[uint64]bool // Indices that must be discarded (Discard Rule)
}

// NewProtocolState creates a new protocol state
func NewProtocolState() *ProtocolState {
	return &ProtocolState{
		CurrentLMSIndex: 0,
		SequenceNumber: 0,
		UnusableIndices: make(map[uint64]bool),
	}
}

// HSMProtocol implements the complete HSM protocol workflow
type HSMProtocol struct {
	client *HSMClient
	state  *ProtocolState
	genesisHash string // Hash of LMS public key + system bundle
}

// NewHSMProtocol creates a new HSM protocol instance
// genesisHash: Hash of LMS public key + system bundle (for genesis entry)
func NewHSMProtocol(client *HSMClient, genesisHash string) *HSMProtocol {
	return &HSMProtocol{
		client:      client,
		state:       NewProtocolState(),
		genesisHash: genesisHash,
	}
}

// GetState returns the current protocol state
func (p *HSMProtocol) GetState() *ProtocolState {
	return p.state
}

// SyncState fetches the latest state from the service and updates local state
// This should be called before generating a new attestation
func (p *HSMProtocol) SyncState() error {
	attestation, raftIndex, raftTerm, err := p.client.GetLatestHead()
	if err != nil {
		return fmt.Errorf("failed to fetch latest head: %v", err)
	}
	
	p.state.LastRaftIndex = raftIndex
	p.state.LastRaftTerm = raftTerm
	
	if attestation == nil {
		// Chain is empty - we're at genesis
		p.state.CurrentLMSIndex = 0
		p.state.SequenceNumber = 0
		p.state.LastAttestation = nil
		return nil
	}
	
	// Extract state from latest attestation
	payload, err := attestation.GetChainedPayload()
	if err != nil {
		return fmt.Errorf("failed to get chained payload: %v", err)
	}
	
	p.state.LastAttestation = attestation
	p.state.CurrentLMSIndex = payload.LMSIndex
	p.state.SequenceNumber = payload.SequenceNumber
	
	return nil
}

// CreateAttestationPayload creates a new attestation payload with correct previous_hash
// This implements the protocol step: "Construct Attestation Payload"
func (p *HSMProtocol) CreateAttestationPayload(
	lmsIndex uint64,
	messageHash string,
	timestamp string,
	metadata string,
) (*models.ChainedPayload, error) {
	// Determine previous_hash
	var previousHash string
	
	if p.state.LastAttestation == nil {
		// This is the genesis entry
		previousHash = p.genesisHash
	} else {
		// Compute hash of previous attestation
		hash, err := p.state.LastAttestation.ComputeHash()
		if err != nil {
			return nil, fmt.Errorf("failed to compute previous hash: %v", err)
		}
		previousHash = hash
	}
	
	// Increment sequence number
	p.state.SequenceNumber++
	
	// Create payload
	payload := &models.ChainedPayload{
		PreviousHash:   previousHash,
		LMSIndex:       lmsIndex,
		MessageSigned:  messageHash,
		SequenceNumber: p.state.SequenceNumber,
		Timestamp:      timestamp,
		Metadata:       metadata,
	}
	
	return payload, nil
}

// CreateAttestationResponse creates a complete AttestationResponse structure
// This is what the HSM would generate after signing the payload
func (p *HSMProtocol) CreateAttestationResponse(
	payload *models.ChainedPayload,
	policyValue string,
	policyAlgorithm string,
	signatureValue string, // Base64-encoded HSM signature
	certificateValue string, // Base64-encoded HSM certificate PEM
) (*models.AttestationResponse, error) {
	attestation := &models.AttestationResponse{}
	
	// Set policy
	attestation.AttestationResponse.Policy.Value = policyValue
	attestation.AttestationResponse.Policy.Algorithm = policyAlgorithm
	
	// Set data (chained payload)
	if err := attestation.SetChainedPayload(payload); err != nil {
		return nil, fmt.Errorf("failed to set chained payload: %v", err)
	}
	
	// Set signature
	attestation.AttestationResponse.Signature.Value = signatureValue
	attestation.AttestationResponse.Signature.Encoding = "base64"
	
	// Set certificate
	attestation.AttestationResponse.Certificate.Value = certificateValue
	attestation.AttestationResponse.Certificate.Encoding = "pem"
	
	return attestation, nil
}

// CommitAttestation submits an attestation to the service and handles the Discard Rule
// This implements the complete "Atomic Log Commitment" phase
// Returns true if committed, false if rejected (and index should be discarded)
func (p *HSMProtocol) CommitAttestation(
	attestation *models.AttestationResponse,
	timeout time.Duration,
) (bool, uint64, uint64, error) {
	// Extract LMS index from attestation
	payload, err := attestation.GetChainedPayload()
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to get chained payload: %v", err)
	}
	
	lmsIndex := payload.LMSIndex
	
	// Submit with timeout
	originalTimeout := p.client.httpClient.Timeout
	p.client.httpClient.Timeout = timeout
	defer func() {
		p.client.httpClient.Timeout = originalTimeout
	}()
	
	committed, raftIndex, raftTerm, err := p.client.ProposeAttestation(attestation)
	
	if err != nil || !committed {
		// Discard Rule: If rejected or timeout, mark index as unusable
		p.state.UnusableIndices[lmsIndex] = true
		return false, 0, 0, fmt.Errorf("attestation rejected or timeout: %v", err)
	}
	
	// Success! Update state
	p.state.LastAttestation = attestation
	p.state.LastRaftIndex = raftIndex
	p.state.LastRaftTerm = raftTerm
	p.state.CurrentLMSIndex = lmsIndex
	p.state.SequenceNumber = payload.SequenceNumber
	
	return true, raftIndex, raftTerm, nil
}

// IsIndexUsable checks if an LMS index is still usable (not discarded)
func (p *HSMProtocol) IsIndexUsable(lmsIndex uint64) bool {
	return !p.state.UnusableIndices[lmsIndex]
}

// GetNextUsableIndex returns the next usable LMS index
// This should be called after SyncState() to determine the next index to use
func (p *HSMProtocol) GetNextUsableIndex() uint64 {
	// Start from current index + 1
	nextIndex := p.state.CurrentLMSIndex + 1
	
	// Skip unusable indices
	for p.state.UnusableIndices[nextIndex] {
		nextIndex++
	}
	
	return nextIndex
}

// ComputeGenesisHash computes the genesis hash from LMS public key and system bundle
// This is a helper function for creating the genesis hash
func ComputeGenesisHash(lmsPublicKey []byte, systemBundle []byte) string {
	hasher := sha256.New()
	hasher.Write(lmsPublicKey)
	hasher.Write(systemBundle)
	hash := hasher.Sum(nil)
	return base64.StdEncoding.EncodeToString(hash)
}

// CompleteWorkflow executes the complete HSM protocol workflow
// This is a convenience method that combines all steps:
// 1. Sync state
// 2. Get next usable index
// 3. Create attestation payload
// 4. Create attestation response (with HSM signature - caller provides this)
// 5. Commit attestation
func (p *HSMProtocol) CompleteWorkflow(
	messageHash string,
	policyValue string,
	policyAlgorithm string,
	signatureValue string,
	certificateValue string,
	timeout time.Duration,
) (bool, uint64, uint64, error) {
	// Step 1: Sync state
	if err := p.SyncState(); err != nil {
		return false, 0, 0, fmt.Errorf("sync state failed: %v", err)
	}
	
	// Step 2: Get next usable index
	lmsIndex := p.GetNextUsableIndex()
	if !p.IsIndexUsable(lmsIndex) {
		return false, 0, 0, fmt.Errorf("index %d is unusable", lmsIndex)
	}
	
	// Step 3: Create attestation payload
	timestamp := time.Now().UTC().Format(time.RFC3339)
	payload, err := p.CreateAttestationPayload(lmsIndex, messageHash, timestamp, "")
	if err != nil {
		return false, 0, 0, fmt.Errorf("create payload failed: %v", err)
	}
	
	// Step 4: Create attestation response (HSM should sign this, but for now we use provided signature)
	attestation, err := p.CreateAttestationResponse(
		payload,
		policyValue,
		policyAlgorithm,
		signatureValue,
		certificateValue,
	)
	if err != nil {
		return false, 0, 0, fmt.Errorf("create attestation failed: %v", err)
	}
	
	// Step 5: Commit attestation
	return p.CommitAttestation(attestation, timeout)
}

