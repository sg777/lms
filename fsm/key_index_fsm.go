package fsm

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/raft"
)

// KeyIndexEntry represents an index commitment for a key_id
type KeyIndexEntry struct {
	KeyID      string `json:"key_id"`
	Index      uint64 `json:"index"`
	Signature  string `json:"signature"`  // Base64 encoded EC signature
	PublicKey  string `json:"public_key"` // Base64 encoded EC public key (for verification)
}

// KeyIndexFSM stores key_id -> index mappings with EC signature verification
type KeyIndexFSM struct {
	mu          sync.RWMutex
	keyIndices  map[string]uint64 // key_id -> last used index
	attestationPubKey *ecdsa.PublicKey // Public key for verifying signatures
}

// NewKeyIndexFSM creates a new key index FSM
// attestationPubKeyPath: Path to the attestation public key PEM file
func NewKeyIndexFSM(attestationPubKeyPath string) (*KeyIndexFSM, error) {
	fsm := &KeyIndexFSM{
		keyIndices: make(map[string]uint64),
	}

	// Load attestation public key
	if attestationPubKeyPath != "" {
		pubKey, err := loadAttestationPublicKey(attestationPubKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load attestation public key: %v", err)
		}
		fsm.attestationPubKey = pubKey
	}

	return fsm, nil
}

func loadAttestationPublicKey(path string) (*ecdsa.PublicKey, error) {
	// Try to load from keys directory first
	keysPath := filepath.Join("./keys", "attestation_public_key.pem")
	if _, err := os.Stat(keysPath); err == nil {
		path = keysPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM")
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	pubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}

	return pubKey, nil
}

// Apply applies a Raft log entry
func (f *KeyIndexFSM) Apply(l *raft.Log) interface{} {
	if l.Type != raft.LogCommand {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	// Try to parse as KeyIndexEntry
	var entry KeyIndexEntry
	if err := json.Unmarshal(l.Data, &entry); err != nil {
		return fmt.Sprintf("Error: Failed to parse key index entry: %v", err)
	}

	// Verify EC signature using the public key from the entry
	if err := f.verifySignature(&entry); err != nil {
		return fmt.Sprintf("Error: Signature verification failed: %v", err)
	}

	// Check if index is valid (must be >= current index for this key_id)
	currentIndex, exists := f.keyIndices[entry.KeyID]
	if exists && entry.Index <= currentIndex {
		return fmt.Sprintf("Error: Index %d is not greater than current index %d for key_id %s",
			entry.Index, currentIndex, entry.KeyID)
	}

	// Store the index
	f.keyIndices[entry.KeyID] = entry.Index

	return fmt.Sprintf("Applied key index: key_id=%s, index=%d", entry.KeyID, entry.Index)
}

func (f *KeyIndexFSM) verifySignature(entry *KeyIndexEntry) error {
	// Create data to verify (key_id + index)
	data := fmt.Sprintf("%s:%d", entry.KeyID, entry.Index)
	hash := sha256.Sum256([]byte(data))
	
	// Debug: log the data being verified (remove in production if needed)
	fmt.Printf("[DEBUG] Verifying signature for data: %s, hash: %x\n", data, hash)

	// Decode signature (base64 encoded ASN.1)
	sigBytes, err := base64.StdEncoding.DecodeString(entry.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}

	// Decode public key from the entry
	pubKeyBytes, err := base64.StdEncoding.DecodeString(entry.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}

	// Parse public key
	pubKeyInterface, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}

	pubKey, ok := pubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("not an ECDSA public key")
	}

	// Verify signature (ASN.1 format) using the public key from the entry
	if !ecdsa.VerifyASN1(pubKey, hash[:], sigBytes) {
		return fmt.Errorf("signature verification failed: ECDSA verify returned false")
	}

	return nil
}

// GetKeyIndex returns the last used index for a key_id
func (f *KeyIndexFSM) GetKeyIndex(keyID string) (uint64, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	index, exists := f.keyIndices[keyID]
	return index, exists
}

// GetAllKeyIndices returns all key_id -> index mappings
func (f *KeyIndexFSM) GetAllKeyIndices() map[string]uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]uint64)
	for k, v := range f.keyIndices {
		result[k] = v
	}
	return result
}

// Snapshot creates a snapshot
func (f *KeyIndexFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return &keyIndexSnapshot{
		keyIndices: f.keyIndices,
	}, nil
}

// Restore restores from snapshot
func (f *KeyIndexFSM) Restore(r io.ReadCloser) error {
	// For now, restore is a no-op
	return nil
}

type keyIndexSnapshot struct {
	keyIndices map[string]uint64
}

func (s *keyIndexSnapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s.keyIndices)
	if err != nil {
		return err
	}
	_, err = sink.Write(data)
	if err != nil {
		return err
	}
	return sink.Close()
}

func (s *keyIndexSnapshot) Release() {}

