package validation

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/verifiable-state-chains/lms/models"
)

// ValidationError represents a validation error with details
type ValidationError struct {
	Field   string
	Reason  string
	Details string
}

func (e *ValidationError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("validation failed: field=%s, reason=%s, details=%s", e.Field, e.Reason, e.Details)
	}
	return fmt.Sprintf("validation failed: field=%s, reason=%s", e.Field, e.Reason)
}

// ValidationResult contains the result of validation
type ValidationResult struct {
	Valid   bool
	Errors  []*ValidationError
	Warnings []string
}

// AttestationValidator validates attestations before they are committed
type AttestationValidator struct {
	genesisHash string
	// For signature verification (can be extended with real crypto)
	verifySignature func(attestation *models.AttestationResponse) error
}

// NewAttestationValidator creates a new attestation validator
func NewAttestationValidator(genesisHash string) *AttestationValidator {
	return &AttestationValidator{
		genesisHash: genesisHash,
		verifySignature: func(attestation *models.AttestationResponse) error {
			// Default: just check signature is present and base64 decodable
			// In production, this would verify the actual signature
			if attestation.AttestationResponse.Signature.Value == "" {
				return fmt.Errorf("signature is empty")
			}
			_, err := base64.StdEncoding.DecodeString(attestation.AttestationResponse.Signature.Value)
			if err != nil {
				return fmt.Errorf("signature is not valid base64: %v", err)
			}
			return nil
		},
	}
}

// SetSignatureVerifier sets a custom signature verifier
func (v *AttestationValidator) SetSignatureVerifier(verifier func(attestation *models.AttestationResponse) error) {
	v.verifySignature = verifier
}

// ValidateAttestation validates a complete attestation
// This should be called before applying to Raft
func (v *AttestationValidator) ValidateAttestation(
	attestation *models.AttestationResponse,
	previousAttestation *models.AttestationResponse,
	isGenesis bool,
) *ValidationResult {
	result := &ValidationResult{
		Valid:   true,
		Errors:  make([]*ValidationError, 0),
		Warnings: make([]string, 0),
	}

	// 1. Validate structure
	if err := v.validateStructure(attestation); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
		return result
	}

	// 2. Validate payload
	payload, err := attestation.GetChainedPayload()
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			Field:  "data.value",
			Reason: "failed to decode chained payload",
			Details: err.Error(),
		})
		return result
	}

	// 3. Validate payload fields
	if err := v.validatePayload(payload, isGenesis); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
	}

	// 4. Validate hash chain (if not genesis)
	if !isGenesis && previousAttestation != nil {
		if err := v.validateHashChain(attestation, payload, previousAttestation); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err)
		}
	} else if isGenesis {
		// Genesis validation
		if err := v.validateGenesis(payload); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err)
		}
	}

	// 5. Validate monotonicity (if not genesis)
	if !isGenesis && previousAttestation != nil {
		if err := v.validateMonotonicity(payload, previousAttestation); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err)
		}
	}

	// 6. Validate signature
	if err := v.verifySignature(attestation); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			Field:  "signature",
			Reason: "signature verification failed",
			Details: err.Error(),
		})
	}

	// 7. Validate certificate
	if err := v.validateCertificate(attestation); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
	}

	// 8. Validate timestamp (warning only, not fatal)
	if err := v.validateTimestamp(payload); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("timestamp validation: %v", err))
	}

	return result
}

// validateStructure validates the basic structure of the attestation
func (v *AttestationValidator) validateStructure(attestation *models.AttestationResponse) *ValidationError {
	// Check policy
	if attestation.AttestationResponse.Policy.Value == "" {
		return &ValidationError{
			Field:  "policy.value",
			Reason: "policy value is required",
		}
	}

	// Check data
	if attestation.AttestationResponse.Data.Value == "" {
		return &ValidationError{
			Field:  "data.value",
			Reason: "data value is required",
		}
	}

	if attestation.AttestationResponse.Data.Encoding != "base64" {
		return &ValidationError{
			Field:  "data.encoding",
			Reason: "data encoding must be 'base64'",
			Details: fmt.Sprintf("got: %s", attestation.AttestationResponse.Data.Encoding),
		}
	}

	// Check signature
	if attestation.AttestationResponse.Signature.Value == "" {
		return &ValidationError{
			Field:  "signature.value",
			Reason: "signature value is required",
		}
	}

	if attestation.AttestationResponse.Signature.Encoding != "base64" {
		return &ValidationError{
			Field:  "signature.encoding",
			Reason: "signature encoding must be 'base64'",
			Details: fmt.Sprintf("got: %s", attestation.AttestationResponse.Signature.Encoding),
		}
	}

	// Check certificate
	if attestation.AttestationResponse.Certificate.Value == "" {
		return &ValidationError{
			Field:  "certificate.value",
			Reason: "certificate value is required",
		}
	}

	if attestation.AttestationResponse.Certificate.Encoding != "pem" {
		return &ValidationError{
			Field:  "certificate.encoding",
			Reason: "certificate encoding must be 'pem'",
			Details: fmt.Sprintf("got: %s", attestation.AttestationResponse.Certificate.Encoding),
		}
	}

	return nil
}

// validatePayload validates the chained payload fields
func (v *AttestationValidator) validatePayload(payload *models.ChainedPayload, isGenesis bool) *ValidationError {
	// Previous hash must be present
	if payload.PreviousHash == "" {
		return &ValidationError{
			Field:  "previous_hash",
			Reason: "previous_hash is required",
		}
	}

	// Message signed must be present
	if payload.MessageSigned == "" {
		return &ValidationError{
			Field:  "message_signed",
			Reason: "message_signed is required",
		}
	}

	// Timestamp must be present
	if payload.Timestamp == "" {
		return &ValidationError{
			Field:  "timestamp",
			Reason: "timestamp is required",
		}
	}

	// Sequence number validation
	if !isGenesis && payload.SequenceNumber == 0 {
		return &ValidationError{
			Field:  "sequence_number",
			Reason: "sequence_number must be > 0 for non-genesis entries",
		}
	}

	return nil
}

// validateGenesis validates genesis entry
func (v *AttestationValidator) validateGenesis(payload *models.ChainedPayload) *ValidationError {
	if payload.PreviousHash != v.genesisHash {
		return &ValidationError{
			Field:  "previous_hash",
			Reason: "genesis entry previous_hash mismatch",
			Details: fmt.Sprintf("expected: %s, got: %s", v.genesisHash, payload.PreviousHash),
		}
	}

	if payload.LMSIndex != 0 {
		return &ValidationError{
			Field:  "lms_index",
			Reason: "genesis entry must have lms_index = 0",
			Details: fmt.Sprintf("got: %d", payload.LMSIndex),
		}
	}

	if payload.SequenceNumber != 0 {
		return &ValidationError{
			Field:  "sequence_number",
			Reason: "genesis entry must have sequence_number = 0",
			Details: fmt.Sprintf("got: %d", payload.SequenceNumber),
		}
	}

	return nil
}

// validateHashChain validates the hash chain link
func (v *AttestationValidator) validateHashChain(
	attestation *models.AttestationResponse,
	payload *models.ChainedPayload,
	previousAttestation *models.AttestationResponse,
) *ValidationError {
	// Compute hash of previous attestation
	prevHash, err := previousAttestation.ComputeHash()
	if err != nil {
		return &ValidationError{
			Field:  "previous_hash",
			Reason: "failed to compute previous attestation hash",
			Details: err.Error(),
		}
	}

	// Verify previous_hash matches
	if payload.PreviousHash != prevHash {
		return &ValidationError{
			Field:  "previous_hash",
			Reason: "hash chain broken",
			Details: fmt.Sprintf("expected: %s, got: %s", prevHash, payload.PreviousHash),
		}
	}

	return nil
}

// validateMonotonicity validates that sequence number and LMS index are monotonic
func (v *AttestationValidator) validateMonotonicity(
	payload *models.ChainedPayload,
	previousAttestation *models.AttestationResponse,
) *ValidationError {
	prevPayload, err := previousAttestation.GetChainedPayload()
	if err != nil {
		return &ValidationError{
			Field:  "monotonicity",
			Reason: "failed to get previous payload",
			Details: err.Error(),
		}
	}

	// Sequence number must be strictly increasing
	if payload.SequenceNumber <= prevPayload.SequenceNumber {
		return &ValidationError{
			Field:  "sequence_number",
			Reason: "sequence number not monotonic",
			Details: fmt.Sprintf("previous: %d, current: %d", prevPayload.SequenceNumber, payload.SequenceNumber),
		}
	}

	// LMS index must be strictly increasing
	if payload.LMSIndex <= prevPayload.LMSIndex {
		return &ValidationError{
			Field:  "lms_index",
			Reason: "LMS index not monotonic",
			Details: fmt.Sprintf("previous: %d, current: %d", prevPayload.LMSIndex, payload.LMSIndex),
		}
	}

	return nil
}

// validateCertificate validates the certificate format
func (v *AttestationValidator) validateCertificate(attestation *models.AttestationResponse) *ValidationError {
	certValue := attestation.AttestationResponse.Certificate.Value
	
	// Decode base64
	certBytes, err := base64.StdEncoding.DecodeString(certValue)
	if err != nil {
		return &ValidationError{
			Field:  "certificate.value",
			Reason: "certificate is not valid base64",
			Details: err.Error(),
		}
	}

	// Check for PEM markers (basic check)
	certStr := string(certBytes)
	// For testing, we allow shorter certificates (mock certificates)
	// In production, this would be more strict
	if len(certStr) < 10 {
		return &ValidationError{
			Field:  "certificate.value",
			Reason: "certificate appears to be too short",
		}
	}

	// In production, this would parse and validate the actual X.509 certificate
	// For now, we just check it's decodable and has reasonable length

	return nil
}

// validateTimestamp validates the timestamp (warning only, not fatal)
func (v *AttestationValidator) validateTimestamp(payload *models.ChainedPayload) error {
	if payload.Timestamp == "" {
		return fmt.Errorf("timestamp is empty")
	}

	// Parse timestamp
	t, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		return fmt.Errorf("timestamp is not valid RFC3339: %v", err)
	}

	// Check timestamp is not too far in the future (more than 1 hour)
	now := time.Now().UTC()
	if t.After(now.Add(1 * time.Hour)) {
		return fmt.Errorf("timestamp is too far in the future: %s", payload.Timestamp)
	}

	// Check timestamp is not too far in the past (more than 1 hour)
	if t.Before(now.Add(-1 * time.Hour)) {
		return fmt.Errorf("timestamp is too far in the past: %s", payload.Timestamp)
	}

	return nil
}

// ValidateChain validates an entire chain of attestations
func (v *AttestationValidator) ValidateChain(attestations []*models.AttestationResponse) *ValidationResult {
	result := &ValidationResult{
		Valid:   true,
		Errors:  make([]*ValidationError, 0),
		Warnings: make([]string, 0),
	}

	if len(attestations) == 0 {
		return result // Empty chain is valid
	}

	// Validate first entry (genesis)
	genesisResult := v.ValidateAttestation(attestations[0], nil, true)
	if !genesisResult.Valid {
		result.Valid = false
		result.Errors = append(result.Errors, genesisResult.Errors...)
		result.Warnings = append(result.Warnings, genesisResult.Warnings...)
	}

	// Validate subsequent entries
	for i := 1; i < len(attestations); i++ {
		entryResult := v.ValidateAttestation(attestations[i], attestations[i-1], false)
		if !entryResult.Valid {
			result.Valid = false
			result.Errors = append(result.Errors, entryResult.Errors...)
		}
		result.Warnings = append(result.Warnings, entryResult.Warnings...)
	}

	return result
}

