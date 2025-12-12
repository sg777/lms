package models

import (
	"encoding/json"
	"time"
)

// LogEntry represents a single entry in the Raft log
// It wraps an AttestationResponse with additional metadata
type LogEntry struct {
	Index           uint64              `json:"index"`            // Raft log index
	Term            uint64              `json:"term"`             // Raft term
	Timestamp       time.Time           `json:"timestamp"`        // When entry was committed
	Attestation     *AttestationResponse `json:"attestation"`     // The attestation data
	CommittedBy     string              `json:"committed_by"`     // HSM identifier that committed this
}

// ToBytes serializes the LogEntry to bytes for Raft storage
func (le *LogEntry) ToBytes() ([]byte, error) {
	return json.Marshal(le)
}

// FromBytes deserializes bytes into LogEntry
func (le *LogEntry) FromBytes(data []byte) error {
	return json.Unmarshal(data, le)
}

// GetPreviousHash extracts the previous_hash from the chained payload
func (le *LogEntry) GetPreviousHash() (string, error) {
	if le.Attestation == nil {
		return "", nil
	}

	payload, err := le.Attestation.GetChainedPayload()
	if err != nil {
		return "", err
	}

	return payload.PreviousHash, nil
}

// GetLMSIndex extracts the LMS index from the chained payload
func (le *LogEntry) GetLMSIndex() (uint64, error) {
	if le.Attestation == nil {
		return 0, nil
	}

	payload, err := le.Attestation.GetChainedPayload()
	if err != nil {
		return 0, err
	}

	return payload.LMSIndex, nil
}

// GetSequenceNumber extracts the sequence number from the chained payload
func (le *LogEntry) GetSequenceNumber() (uint64, error) {
	if le.Attestation == nil {
		return 0, nil
	}

	payload, err := le.Attestation.GetChainedPayload()
	if err != nil {
		return 0, err
	}

	return payload.SequenceNumber, nil
}

