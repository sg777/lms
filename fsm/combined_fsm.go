package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/models"
)

// CombinedFSM combines HashChainFSM and KeyIndexFSM
type CombinedFSM struct {
	mu          sync.RWMutex
	hashChainFSM *HashChainFSM
	keyIndexFSM  *KeyIndexFSM
}

// NewCombinedFSM creates a new combined FSM
func NewCombinedFSM(genesisHash string, attestationPubKeyPath string) (*CombinedFSM, error) {
	hashChainFSM := NewHashChainFSM(genesisHash)
	
	keyIndexFSM, err := NewKeyIndexFSM(attestationPubKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create key index FSM: %v", err)
	}
	
	return &CombinedFSM{
		hashChainFSM: hashChainFSM,
		keyIndexFSM:  keyIndexFSM,
	}, nil
}

// Apply applies a Raft log entry
func (f *CombinedFSM) Apply(l *raft.Log) interface{} {
	if l.Type != raft.LogCommand {
		return nil
	}

	// Try to parse as KeyIndexEntry first
	var keyIndexEntry KeyIndexEntry
	if err := json.Unmarshal(l.Data, &keyIndexEntry); err == nil && keyIndexEntry.KeyID != "" {
		// It's a key index entry
		return f.keyIndexFSM.Apply(l)
	}

	// Otherwise, treat as hash-chain attestation
	return f.hashChainFSM.Apply(l)
}

// Snapshot creates a snapshot
func (f *CombinedFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	
	hashChainSnapshot, err := f.hashChainFSM.Snapshot()
	if err != nil {
		return nil, err
	}
	
	keyIndexSnapshot, err := f.keyIndexFSM.Snapshot()
	if err != nil {
		return nil, err
	}
	
	return &combinedSnapshot{
		hashChainSnapshot: hashChainSnapshot,
		keyIndexSnapshot:   keyIndexSnapshot,
	}, nil
}

// Restore restores from snapshot
func (f *CombinedFSM) Restore(r io.ReadCloser) error {
	// For now, restore is a no-op
	return nil
}

// HashChainFSM methods
func (f *CombinedFSM) GetLatestAttestation() (*models.AttestationResponse, error) {
	return f.hashChainFSM.GetLatestAttestation()
}

func (f *CombinedFSM) GetLogEntry(index uint64) (*models.LogEntry, error) {
	return f.hashChainFSM.GetLogEntry(index)
}

func (f *CombinedFSM) GetLogCount() uint64 {
	return f.hashChainFSM.GetLogCount()
}

func (f *CombinedFSM) GetSimpleMessages() []string {
	return f.hashChainFSM.GetSimpleMessages()
}

func (f *CombinedFSM) GetAllLogEntries() []*models.LogEntry {
	return f.hashChainFSM.GetAllLogEntries()
}

func (f *CombinedFSM) GetGenesisHash() string {
	return f.hashChainFSM.GetGenesisHash()
}

// KeyIndexFSM methods
func (f *CombinedFSM) GetKeyIndex(keyID string) (uint64, bool) {
	return f.keyIndexFSM.GetKeyIndex(keyID)
}

func (f *CombinedFSM) GetKeyHash(keyID string) (string, bool) {
	return f.keyIndexFSM.GetKeyHash(keyID)
}

func (f *CombinedFSM) GetAllKeyIndices() map[string]uint64 {
	return f.keyIndexFSM.GetAllKeyIndices()
}

func (f *CombinedFSM) GetAllKeyIDs() []string {
	return f.keyIndexFSM.GetAllKeyIDs()
}

func (f *CombinedFSM) GetKeyChain(keyID string) ([]*KeyIndexEntry, bool) {
	return f.keyIndexFSM.GetKeyChain(keyID)
}

func (f *CombinedFSM) GetIndexAndHashByPubkeyHash(pubkeyHash string) (uint64, string, bool) {
	return f.keyIndexFSM.GetIndexAndHashByPubkeyHash(pubkeyHash)
}

func (f *CombinedFSM) GetChainByPubkeyHash(pubkeyHash string) ([]*KeyIndexEntry, bool) {
	return f.keyIndexFSM.GetChainByPubkeyHash(pubkeyHash)
}

func (f *CombinedFSM) GetAllEntries(limit int) []struct {
	Entry     *KeyIndexEntry
	RaftIndex uint64
} {
	return f.keyIndexFSM.GetAllEntries(limit)
}

func (f *CombinedFSM) GetAllPubkeyHashesByKeyID(keyID string) []string {
	return f.keyIndexFSM.GetAllPubkeyHashesByKeyID(keyID)
}

type combinedSnapshot struct {
	hashChainSnapshot raft.FSMSnapshot
	keyIndexSnapshot  raft.FSMSnapshot
}

func (s *combinedSnapshot) Persist(sink raft.SnapshotSink) error {
	// For now, just persist hash chain
	return s.hashChainSnapshot.Persist(sink)
}

func (s *combinedSnapshot) Release() {
	s.hashChainSnapshot.Release()
	s.keyIndexSnapshot.Release()
}

