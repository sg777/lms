package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/models"
)

// HashChainFSM implements a hash-chain based FSM for storing LMS attestations
// It maintains a cryptographically linked chain where each entry's previous_hash
// must match the hash of the previous entry
type HashChainFSM struct {
	mu             sync.RWMutex
	attestations   []*models.AttestationResponse
	logEntries     []*models.LogEntry
	simpleMessages []string // Store simple string messages separately
	genesisHash    string   // Hash of the genesis block (LMS public key + system bundle)
}

// NewHashChainFSM creates a new hash-chain FSM
func NewHashChainFSM(genesisHash string) *HashChainFSM {
	return &HashChainFSM{
		attestations:   make([]*models.AttestationResponse, 0),
		logEntries:    make([]*models.LogEntry, 0),
		simpleMessages: make([]string, 0),
		genesisHash:   genesisHash,
	}
}

// Apply applies a Raft log entry to the FSM
func (f *HashChainFSM) Apply(l *raft.Log) interface{} {
	if l.Type != raft.LogCommand {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Try to parse as attestation first
	var attestation models.AttestationResponse
	if err := json.Unmarshal(l.Data, &attestation); err == nil {
		// It's an attestation - validate hash chain
		payload, err := attestation.GetChainedPayload()
		if err != nil {
			return fmt.Sprintf("Error: Failed to get chained payload: %v", err)
		}

		// Validate hash chain integrity
		if err := f.validateHashChain(&attestation, payload); err != nil {
			return fmt.Sprintf("Error: Hash chain validation failed: %v", err)
		}

		// Create log entry
		entry := &models.LogEntry{
			Index:       uint64(l.Index),
			Term:        uint64(l.Term),
			Attestation:  &attestation,
		}

		// Store the attestation and log entry
		f.attestations = append(f.attestations, &attestation)
		f.logEntries = append(f.logEntries, entry)

		return fmt.Sprintf("Applied attestation: index=%d, lms_index=%d, sequence=%d",
			l.Index, payload.LMSIndex, payload.SequenceNumber)
	}

	// If not an attestation, treat as simple string message (for testing)
	message := string(l.Data)
	fmt.Printf("Applied log: %s\n", message)
	
	// Store the simple message
	f.simpleMessages = append(f.simpleMessages, message)
	
	// Create a simple log entry for string messages
	entry := &models.LogEntry{
		Index:       uint64(l.Index),
		Term:        uint64(l.Term),
		Attestation:  nil, // No attestation for simple messages
	}
	f.logEntries = append(f.logEntries, entry)

	return fmt.Sprintf("Stored log: %s", message)
}

// validateHashChain validates that the previous_hash in the new attestation
// matches the hash of the previous entry in the chain
func (f *HashChainFSM) validateHashChain(attestation *models.AttestationResponse, payload *models.ChainedPayload) error {
	// If this is the first entry (genesis), previous_hash should match genesis hash
	if len(f.attestations) == 0 {
		if payload.PreviousHash != f.genesisHash {
			return fmt.Errorf("genesis entry previous_hash mismatch: expected %s, got %s",
				f.genesisHash, payload.PreviousHash)
		}
		return nil
	}

	// Get the previous attestation
	prevAttestation := f.attestations[len(f.attestations)-1]

	// Compute hash of previous attestation
	prevHash, err := prevAttestation.ComputeHash()
	if err != nil {
		return fmt.Errorf("failed to compute previous hash: %v", err)
	}

	// Verify previous_hash matches
	if payload.PreviousHash != prevHash {
		return fmt.Errorf("hash chain broken: expected previous_hash %s, got %s",
			prevHash, payload.PreviousHash)
	}

	// Verify sequence number is monotonic
	prevPayload, err := prevAttestation.GetChainedPayload()
	if err != nil {
		return fmt.Errorf("failed to get previous payload: %v", err)
	}

	if payload.SequenceNumber <= prevPayload.SequenceNumber {
		return fmt.Errorf("sequence number not monotonic: previous=%d, current=%d",
			prevPayload.SequenceNumber, payload.SequenceNumber)
	}

	// Verify LMS index is monotonic
	if payload.LMSIndex <= prevPayload.LMSIndex {
		return fmt.Errorf("LMS index not monotonic: previous=%d, current=%d",
			prevPayload.LMSIndex, payload.LMSIndex)
	}

	return nil
}

// Snapshot creates a snapshot of the FSM state
func (f *HashChainFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &hashChainSnapshot{
		attestations: f.attestations,
		logEntries:   f.logEntries,
		genesisHash:  f.genesisHash,
	}, nil
}

// Restore restores the FSM from a snapshot
func (f *HashChainFSM) Restore(r io.ReadCloser) error {
	// For now, restore is a no-op
	// In a production system, this would deserialize from the snapshot
	// Will be enhanced in future if needed
	return nil
}

// GetLatestAttestation returns the latest committed attestation
func (f *HashChainFSM) GetLatestAttestation() (*models.AttestationResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.attestations) == 0 {
		return nil, fmt.Errorf("no attestations committed yet")
	}

	// Return a copy to avoid race conditions
	latest := f.attestations[len(f.attestations)-1]
	
	// Create a deep copy by serializing and deserializing
	data, err := latest.ToJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize attestation: %v", err)
	}

	var copy models.AttestationResponse
	if err := copy.FromJSON(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize attestation: %v", err)
	}

	return &copy, nil
}

// GetLogEntry returns a log entry by Raft index (1-based)
func (f *HashChainFSM) GetLogEntry(index uint64) (*models.LogEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if index == 0 || index > uint64(len(f.logEntries)) {
		return nil, fmt.Errorf("invalid log index: %d (valid range: 1-%d)",
			index, len(f.logEntries))
	}

	entry := f.logEntries[index-1]
	
	// Return a copy
	data, err := entry.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize log entry: %v", err)
	}

	var copy models.LogEntry
	if err := copy.FromBytes(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize log entry: %v", err)
	}

	return &copy, nil
}

// GetLogCount returns the number of committed log entries
func (f *HashChainFSM) GetLogCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return uint64(len(f.logEntries))
}

// GetAllLogs returns all log entries (for simple string messages)
func (f *HashChainFSM) GetAllLogs() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	logs := make([]string, 0, len(f.logEntries))
	for _, entry := range f.logEntries {
		if entry.Attestation == nil {
			// Simple string message - extract from Raft log
			// We need to store this differently, for now return empty
			continue
		}
		// For attestations, we could return summary
		// For now, return empty for attestations
	}
	return logs
}

// GetAllLogEntries returns all log entries with full details
func (f *HashChainFSM) GetAllLogEntries() []*models.LogEntry {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Return copies to avoid race conditions
	entries := make([]*models.LogEntry, 0, len(f.logEntries))
	for _, entry := range f.logEntries {
		data, err := entry.ToBytes()
		if err != nil {
			continue
		}
		var copy models.LogEntry
		if err := copy.FromBytes(data); err != nil {
			continue
		}
		entries = append(entries, &copy)
	}
	return entries
}

// GetSimpleMessages returns all simple string messages (non-attestation logs)
func (f *HashChainFSM) GetSimpleMessages() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	// Return a copy
	messages := make([]string, len(f.simpleMessages))
	copy(messages, f.simpleMessages)
	return messages
}

// GetChainHeadHash returns the hash of the latest attestation (chain head)
func (f *HashChainFSM) GetChainHeadHash() (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.attestations) == 0 {
		return f.genesisHash, nil
	}

	latest := f.attestations[len(f.attestations)-1]
	return latest.ComputeHash()
}

// GetGenesisHash returns the genesis hash
func (f *HashChainFSM) GetGenesisHash() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.genesisHash
}

// VerifyChainIntegrity verifies the integrity of the entire chain
func (f *HashChainFSM) VerifyChainIntegrity() error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.attestations) == 0 {
		return nil // Empty chain is valid
	}

	// Start from genesis
	expectedPrevHash := f.genesisHash

	for i, attestation := range f.attestations {
		payload, err := attestation.GetChainedPayload()
		if err != nil {
			return fmt.Errorf("entry %d: failed to get payload: %v", i, err)
		}

		// Verify previous hash
		if payload.PreviousHash != expectedPrevHash {
			return fmt.Errorf("entry %d: hash chain broken: expected %s, got %s",
				i, expectedPrevHash, payload.PreviousHash)
		}

		// Compute hash for next iteration
		hash, err := attestation.ComputeHash()
		if err != nil {
			return fmt.Errorf("entry %d: failed to compute hash: %v", i, err)
		}
		expectedPrevHash = hash

		// Verify sequence monotonicity (except for first entry)
		if i > 0 {
			prevPayload, err := f.attestations[i-1].GetChainedPayload()
			if err != nil {
				return fmt.Errorf("entry %d: failed to get previous payload: %v", i, err)
			}

			if payload.SequenceNumber <= prevPayload.SequenceNumber {
				return fmt.Errorf("entry %d: sequence not monotonic: %d <= %d",
					i, payload.SequenceNumber, prevPayload.SequenceNumber)
			}

			if payload.LMSIndex <= prevPayload.LMSIndex {
				return fmt.Errorf("entry %d: LMS index not monotonic: %d <= %d",
					i, payload.LMSIndex, prevPayload.LMSIndex)
			}
		}
	}

	return nil
}

// hashChainSnapshot represents a snapshot of the FSM state
type hashChainSnapshot struct {
	attestations []*models.AttestationResponse
	logEntries   []*models.LogEntry
	genesisHash  string
}

func (s *hashChainSnapshot) Persist(sink raft.SnapshotSink) error {
	// Serialize snapshot data
	snapshotData := struct {
		Attestations []*models.AttestationResponse `json:"attestations"`
		LogEntries   []*models.LogEntry            `json:"log_entries"`
		GenesisHash  string                        `json:"genesis_hash"`
	}{
		Attestations: s.attestations,
		LogEntries:   s.logEntries,
		GenesisHash:  s.genesisHash,
	}

	data, err := json.Marshal(snapshotData)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}

	if _, err := sink.Write(data); err != nil {
		return fmt.Errorf("failed to write snapshot: %v", err)
	}

	return sink.Close()
}

func (s *hashChainSnapshot) Release() {
	// No cleanup needed for in-memory snapshot
}

