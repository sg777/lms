package models

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"time"
)

// ChainedPayload represents the core attestation data that forms the hash chain
type ChainedPayload struct {
	PreviousHash   string `json:"previous_hash"`   // SHA-256 hash of previous attestation response
	LMSIndex       uint64 `json:"lms_index"`       // Current LMS index being used
	MessageSigned  string `json:"message_signed"`  // Hash of the message being signed
	SequenceNumber uint64 `json:"sequence_number"` // Monotonically increasing sequence number
	Timestamp      string `json:"timestamp"`       // HSM's secure timestamp
	Metadata       string `json:"metadata"`        // Additional metadata (optional)
}

// AttestationResponse represents the complete attestation response structure
// This matches the format described in the paper (Section 4.3)
type AttestationResponse struct {
	AttestationResponse struct {
		Policy struct {
			Value     string `json:"value"`     // "LMS_ATTEST_POLICY"
			Algorithm string `json:"algorithm"` // "PS256" or HSM's internal mechanism
		} `json:"policy"`
		Data struct {
			Value    string `json:"value"`    // Base64-encoded chained payload
			Encoding string `json:"encoding"` // "base64"
		} `json:"data"`
		Signature struct {
			Value    string `json:"value"`    // Base64-encoded HSM attestation signature
			Encoding string `json:"encoding"` // "base64"
		} `json:"signature"`
		Certificate struct {
			Value    string `json:"value"`    // Base64-encoded HSM attestation certificate PEM
			Encoding string `json:"encoding"` // "pem"
		} `json:"certificate"`
	} `json:"attestation_response"`
}

// ToJSON serializes the AttestationResponse to JSON
func (ar *AttestationResponse) ToJSON() ([]byte, error) {
	return json.Marshal(ar)
}

// FromJSON deserializes JSON into AttestationResponse
func (ar *AttestationResponse) FromJSON(data []byte) error {
	return json.Unmarshal(data, ar)
}

// GetChainedPayload decodes and returns the ChainedPayload from the attestation
func (ar *AttestationResponse) GetChainedPayload() (*ChainedPayload, error) {
	encoded := ar.AttestationResponse.Data.Value
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	var payload ChainedPayload
	if err := json.Unmarshal(decoded, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

// SetChainedPayload encodes and sets the ChainedPayload in the attestation
func (ar *AttestationResponse) SetChainedPayload(payload *ChainedPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ar.AttestationResponse.Data.Value = base64.StdEncoding.EncodeToString(jsonData)
	ar.AttestationResponse.Data.Encoding = "base64"
	return nil
}

// ComputeHash computes the SHA-256 hash of the entire attestation response
// This is used to create the hash chain link
func (ar *AttestationResponse) ComputeHash() (string, error) {
	jsonData, err := ar.ToJSON()
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(jsonData)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// CreateGenesisPayload creates the initial chained payload for index 0
// previous_hash is set to hash of LMS public key and system bundle
func CreateGenesisPayload(lmsPublicKeyHash string, lmsIndex uint64, messageHash string) *ChainedPayload {
	return &ChainedPayload{
		PreviousHash:   lmsPublicKeyHash, // Genesis hash
		LMSIndex:       lmsIndex,
		MessageSigned:  messageHash,
		SequenceNumber: 0,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Metadata:       "genesis",
	}
}

