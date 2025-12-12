package service

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/models"
)

// SimpleFSM is a temporary FSM for testing Module 2
// It will be replaced by the hash-chain FSM in Module 3
type SimpleFSM struct {
	mu            sync.RWMutex
	attestations  []*models.AttestationResponse
	logEntries    []*models.LogEntry
}

// NewSimpleFSM creates a new simple FSM
func NewSimpleFSM() *SimpleFSM {
	return &SimpleFSM{
		attestations: make([]*models.AttestationResponse, 0),
		logEntries:   make([]*models.LogEntry, 0),
	}
}

// Apply applies a Raft log entry
func (f *SimpleFSM) Apply(l *raft.Log) interface{} {
	if l.Type != raft.LogCommand {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Try to deserialize as AttestationResponse
	var attestation models.AttestationResponse
	if err := json.Unmarshal(l.Data, &attestation); err != nil {
		return fmt.Sprintf("Failed to parse attestation: %v", err)
	}

	// Create log entry
	entry := &models.LogEntry{
		Index:       uint64(l.Index),
		Term:         uint64(l.Term),
		Attestation:  &attestation,
	}

	f.attestations = append(f.attestations, &attestation)
	f.logEntries = append(f.logEntries, entry)

	return fmt.Sprintf("Applied attestation at index %d", l.Index)
}

// Snapshot returns a snapshot of the FSM
func (f *SimpleFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return &simpleFSMSnapshot{
		attestations: f.attestations,
		logEntries:   f.logEntries,
	}, nil
}

// Restore restores the FSM from a snapshot
func (f *SimpleFSM) Restore(r io.ReadCloser) error {
	// For now, restore is a no-op
	// Will be implemented properly in Module 3
	return nil
}

// GetLatestAttestation returns the latest attestation
func (f *SimpleFSM) GetLatestAttestation() (*models.AttestationResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(f.attestations) == 0 {
		return nil, fmt.Errorf("no attestations yet")
	}

	return f.attestations[len(f.attestations)-1], nil
}

// GetLogEntry returns a log entry by index
func (f *SimpleFSM) GetLogEntry(index uint64) (*models.LogEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if index == 0 || index > uint64(len(f.logEntries)) {
		return nil, fmt.Errorf("invalid log index: %d", index)
	}

	return f.logEntries[index-1], nil
}

// GetLogCount returns the number of log entries
func (f *SimpleFSM) GetLogCount() uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return uint64(len(f.logEntries))
}

// simpleFSMSnapshot is a snapshot of the simple FSM
type simpleFSMSnapshot struct {
	attestations []*models.AttestationResponse
	logEntries   []*models.LogEntry
}

func (s *simpleFSMSnapshot) Persist(sink raft.SnapshotSink) error {
	// For now, persist is a no-op
	return nil
}

func (s *simpleFSMSnapshot) Release() {
	// No-op
}

