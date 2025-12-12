package validation

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/verifiable-state-chains/lms/models"
)

func TestAttestationValidator_ValidateStructure(t *testing.T) {
	validator := NewAttestationValidator("genesis-hash")

	// Test empty attestation
	attestation := &models.AttestationResponse{}
	result := validator.ValidateAttestation(attestation, nil, true)
	if result.Valid {
		t.Error("Empty attestation should be invalid")
	}

	// Test missing policy
	attestation = &models.AttestationResponse{}
	attestation.AttestationResponse.Data.Value = "dGVzdA=="
	attestation.AttestationResponse.Data.Encoding = "base64"
	result = validator.ValidateAttestation(attestation, nil, true)
	if result.Valid {
		t.Error("Attestation without policy should be invalid")
	}

	// Test missing signature
	attestation = &models.AttestationResponse{}
	attestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	attestation.AttestationResponse.Data.Value = "dGVzdA=="
	attestation.AttestationResponse.Data.Encoding = "base64"
	result = validator.ValidateAttestation(attestation, nil, true)
	if result.Valid {
		t.Error("Attestation without signature should be invalid")
	}
}

func TestAttestationValidator_ValidateGenesis(t *testing.T) {
	genesisHash := "genesis-hash-123"
	validator := NewAttestationValidator(genesisHash)

	// Create valid genesis payload
	payload := &models.ChainedPayload{
		PreviousHash:   genesisHash,
		LMSIndex:       0,
		MessageSigned:  "message-hash",
		SequenceNumber: 0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	attestation := &models.AttestationResponse{}
	attestation.SetChainedPayload(payload)
	attestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	attestation.AttestationResponse.Policy.Algorithm = "PS256"
	attestation.AttestationResponse.Data.Encoding = "base64"
	attestation.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("signature"))
	attestation.AttestationResponse.Signature.Encoding = "base64"
	attestation.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----"))
	attestation.AttestationResponse.Certificate.Encoding = "pem"

	result := validator.ValidateAttestation(attestation, nil, true)
	if !result.Valid {
		t.Errorf("Valid genesis attestation should pass: %v", result.Errors)
	}

	// Test wrong genesis hash
	payload.PreviousHash = "wrong-hash"
	attestation.SetChainedPayload(payload)
	result = validator.ValidateAttestation(attestation, nil, true)
	if result.Valid {
		t.Error("Genesis with wrong previous_hash should be invalid")
	}

	// Test non-zero LMS index
	payload.PreviousHash = genesisHash
	payload.LMSIndex = 1
	attestation.SetChainedPayload(payload)
	result = validator.ValidateAttestation(attestation, nil, true)
	if result.Valid {
		t.Error("Genesis with non-zero LMS index should be invalid")
	}
}

func TestAttestationValidator_ValidateHashChain(t *testing.T) {
	genesisHash := "genesis-hash-123"
	validator := NewAttestationValidator(genesisHash)

	// Create genesis
	genesisPayload := &models.ChainedPayload{
		PreviousHash:   genesisHash,
		LMSIndex:       0,
		MessageSigned:  "message-0",
		SequenceNumber: 0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	genesis := &models.AttestationResponse{}
	genesis.SetChainedPayload(genesisPayload)
	genesis.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	genesis.AttestationResponse.Data.Encoding = "base64"
	genesis.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("sig"))
	genesis.AttestationResponse.Signature.Encoding = "base64"
	genesis.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("cert"))
	genesis.AttestationResponse.Certificate.Encoding = "pem"

	// Compute genesis hash
	genesisHashComputed, _ := genesis.ComputeHash()

	// Create next entry with correct previous_hash
	nextPayload := &models.ChainedPayload{
		PreviousHash:   genesisHashComputed,
		LMSIndex:       1,
		MessageSigned:  "message-1",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	next := &models.AttestationResponse{}
	next.SetChainedPayload(nextPayload)
	next.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	next.AttestationResponse.Data.Encoding = "base64"
	next.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("sig"))
	next.AttestationResponse.Signature.Encoding = "base64"
	next.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----"))
	next.AttestationResponse.Certificate.Encoding = "pem"

	result := validator.ValidateAttestation(next, genesis, false)
	if !result.Valid {
		t.Errorf("Valid hash chain should pass: %v", result.Errors)
	}

	// Test broken hash chain
	nextPayload.PreviousHash = "wrong-hash"
	next.SetChainedPayload(nextPayload)
	result = validator.ValidateAttestation(next, genesis, false)
	if result.Valid {
		t.Error("Broken hash chain should be invalid")
	}
}

func TestAttestationValidator_ValidateMonotonicity(t *testing.T) {
	genesisHash := "genesis-hash"
	validator := NewAttestationValidator(genesisHash)

	// Create previous entry
	prevPayload := &models.ChainedPayload{
		PreviousHash:   genesisHash,
		LMSIndex:       5,
		MessageSigned:  "message",
		SequenceNumber: 10,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	prev := &models.AttestationResponse{}
	prev.SetChainedPayload(prevPayload)
	prevHash, _ := prev.ComputeHash()
	prev.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	prev.AttestationResponse.Data.Encoding = "base64"
	prev.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("sig"))
	prev.AttestationResponse.Signature.Encoding = "base64"
	prev.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----"))
	prev.AttestationResponse.Certificate.Encoding = "pem"

	// Recompute prev hash after setting all fields
	prevHash, _ = prev.ComputeHash()

	// Test valid monotonic increase
	nextPayload := &models.ChainedPayload{
		PreviousHash:   prevHash,
		LMSIndex:       6,
		MessageSigned:  "message",
		SequenceNumber: 11,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	next := &models.AttestationResponse{}
	next.SetChainedPayload(nextPayload)
	next.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	next.AttestationResponse.Data.Encoding = "base64"
	next.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("sig"))
	next.AttestationResponse.Signature.Encoding = "base64"
	next.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----"))
	next.AttestationResponse.Certificate.Encoding = "pem"

	result := validator.ValidateAttestation(next, prev, false)
	if !result.Valid {
		t.Errorf("Valid monotonic increase should pass: %v", result.Errors)
	}

	// Test non-monotonic sequence number
	nextPayload.SequenceNumber = 10 // Same as previous
	next.SetChainedPayload(nextPayload)
	result = validator.ValidateAttestation(next, prev, false)
	if result.Valid {
		t.Error("Non-monotonic sequence number should be invalid")
	}

	// Test non-monotonic LMS index
	nextPayload.SequenceNumber = 11
	nextPayload.LMSIndex = 5 // Same as previous
	next.SetChainedPayload(nextPayload)
	result = validator.ValidateAttestation(next, prev, false)
	if result.Valid {
		t.Error("Non-monotonic LMS index should be invalid")
	}
}

func TestAttestationValidator_ValidateChain(t *testing.T) {
	genesisHash := "genesis-hash"
	validator := NewAttestationValidator(genesisHash)

	// Create a valid chain
	genesisPayload := &models.ChainedPayload{
		PreviousHash:   genesisHash,
		LMSIndex:       0,
		MessageSigned:  "message-0",
		SequenceNumber: 0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	genesis := &models.AttestationResponse{}
	genesis.SetChainedPayload(genesisPayload)
	genesis.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	genesis.AttestationResponse.Data.Encoding = "base64"
	genesis.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("sig"))
	genesis.AttestationResponse.Signature.Encoding = "base64"
	genesis.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----"))
	genesis.AttestationResponse.Certificate.Encoding = "pem"

	genesisHashComputed, _ := genesis.ComputeHash()

	nextPayload := &models.ChainedPayload{
		PreviousHash:   genesisHashComputed,
		LMSIndex:       1,
		MessageSigned:  "message-1",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	next := &models.AttestationResponse{}
	next.SetChainedPayload(nextPayload)
	next.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	next.AttestationResponse.Data.Encoding = "base64"
	next.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("sig"))
	next.AttestationResponse.Signature.Encoding = "base64"
	next.AttestationResponse.Certificate.Value = base64.StdEncoding.EncodeToString([]byte("-----BEGIN CERTIFICATE-----\ncertificate\n-----END CERTIFICATE-----"))
	next.AttestationResponse.Certificate.Encoding = "pem"

	chain := []*models.AttestationResponse{genesis, next}
	result := validator.ValidateChain(chain)
	if !result.Valid {
		t.Errorf("Valid chain should pass: %v", result.Errors)
	}
}

func TestMockSignatureVerifier(t *testing.T) {
	verifier := MockSignatureVerifier()

	attestation := &models.AttestationResponse{}
	attestation.AttestationResponse.Signature.Value = base64.StdEncoding.EncodeToString([]byte("signature"))

	err := verifier(attestation)
	if err != nil {
		t.Errorf("Mock verifier should accept valid base64 signature: %v", err)
	}

	// Test empty signature
	attestation.AttestationResponse.Signature.Value = ""
	err = verifier(attestation)
	if err == nil {
		t.Error("Mock verifier should reject empty signature")
	}

	// Test invalid base64
	attestation.AttestationResponse.Signature.Value = "invalid-base64!!!"
	err = verifier(attestation)
	if err == nil {
		t.Error("Mock verifier should reject invalid base64")
	}
}

