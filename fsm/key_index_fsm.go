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

// KeyIndexEntry represents an index commitment for a key_id with hash chain
type KeyIndexEntry struct {
	KeyID       string `json:"key_id"`
	Index       uint64 `json:"index"`
	PreviousHash string `json:"previous_hash"` // SHA-256 hash of the previous entry (genesis: all 0's)
	Hash        string `json:"hash"`           // SHA-256 hash of this entry (computed on all fields except hash)
	Signature   string `json:"signature"`      // Base64 encoded EC signature
	PublicKey   string `json:"public_key"`     // Base64 encoded EC public key (for verification)
}

// ComputeHash computes the SHA-256 hash of the entry
// Hash is computed on all fields EXCEPT the Hash field itself
func (e *KeyIndexEntry) ComputeHash() (string, error) {
	// Create a temporary entry without the Hash field for computing hash
	tempEntry := struct {
		KeyID       string `json:"key_id"`
		Index       uint64 `json:"index"`
		PreviousHash string `json:"previous_hash"`
		Signature   string `json:"signature"`
		PublicKey   string `json:"public_key"`
	}{
		KeyID:       e.KeyID,
		Index:       e.Index,
		PreviousHash: e.PreviousHash,
		Signature:   e.Signature,
		PublicKey:   e.PublicKey,
	}

	jsonData, err := json.Marshal(tempEntry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal entry for hashing: %v", err)
	}

	hash := sha256.Sum256(jsonData)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// GenesisHash is the hash used for the first entry (all zeros)
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// KeyIndexFSM stores key_id -> index mappings with EC signature verification and hash chain
type KeyIndexFSM struct {
	mu               sync.RWMutex
	keyIndices       map[string]uint64              // key_id -> last used index
	keyHashes        map[string]string              // key_id -> hash of last entry (for hash chain validation)
	keyEntries       map[string][]*KeyIndexEntry     // key_id -> all entries (for full chain retrieval)
	attestationPubKey *ecdsa.PublicKey             // Public key for verifying signatures
}

// NewKeyIndexFSM creates a new key index FSM
// attestationPubKeyPath: Path to the attestation public key PEM file
func NewKeyIndexFSM(attestationPubKeyPath string) (*KeyIndexFSM, error) {
	fsm := &KeyIndexFSM{
		keyIndices: make(map[string]uint64),
		keyHashes:  make(map[string]string),
		keyEntries: make(map[string][]*KeyIndexEntry),
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

	// Validate hash chain integrity
	if err := f.validateHashChain(&entry); err != nil {
		return fmt.Sprintf("Error: Hash chain validation failed: %v", err)
	}

	// Compute and verify hash
	computedHash, err := entry.ComputeHash()
	if err != nil {
		return fmt.Sprintf("Error: Failed to compute hash: %v", err)
	}

	if entry.Hash != computedHash {
		return fmt.Sprintf("Error: Hash mismatch: expected %s, got %s", computedHash, entry.Hash)
	}

	// Check if index is valid (must be > current index for this key_id)
	currentIndex, exists := f.keyIndices[entry.KeyID]
	if exists && entry.Index <= currentIndex {
		return fmt.Sprintf("Error: Index %d is not greater than current index %d for key_id %s",
			entry.Index, currentIndex, entry.KeyID)
	}

	// Store the index and hash (the hash is the actual hash of this commit, computed above)
	// This stored hash will be used as previous_hash for the next entry - never recomputed
	f.keyIndices[entry.KeyID] = entry.Index
	f.keyHashes[entry.KeyID] = entry.Hash
	
	// Store the full entry for chain retrieval
	entryCopy := &KeyIndexEntry{
		KeyID:       entry.KeyID,
		Index:       entry.Index,
		PreviousHash: entry.PreviousHash,
		Hash:        entry.Hash,
		Signature:   entry.Signature,
		PublicKey:   entry.PublicKey,
	}
	f.keyEntries[entry.KeyID] = append(f.keyEntries[entry.KeyID], entryCopy)

	return fmt.Sprintf("Applied key index: key_id=%s, index=%d, hash=%s", entry.KeyID, entry.Index, entry.Hash)
}

// VerifySignature verifies the signature of a key index entry
// Made public for testing
func (f *KeyIndexFSM) VerifySignature(entry *KeyIndexEntry) error {
	return f.verifySignature(entry)
}

// VerifyCommitSignature verifies a commit signature matches the expected attestation public key
// This should be called BEFORE applying to Raft to reject unauthorized commits
// Only the HSM server with the attestation private key should be able to commit
func VerifyCommitSignature(entry *KeyIndexEntry) error {
	// This is a helper function that can be called from API server
	// Load the expected attestation public key
	keysPath := filepath.Join("./keys", "attestation_public_key.pem")
	pubKey, err := loadAttestationPublicKey(keysPath)
	if err != nil {
		return fmt.Errorf("failed to load expected attestation public key: %v", err)
	}

	// Create data to verify (key_id:index format)
	data := fmt.Sprintf("%s:%d", entry.KeyID, entry.Index)
	hash := sha256.Sum256([]byte(data))

	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(entry.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}

	// Decode public key from request
	reqPubKeyBytes, err := base64.StdEncoding.DecodeString(entry.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to decode public key from request: %v", err)
	}

	// Parse public key from request
	reqPubKeyInterface, err := x509.ParsePKIXPublicKey(reqPubKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key from request: %v", err)
	}

	reqPubKey, ok := reqPubKeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key from request is not ECDSA")
	}

	// CRITICAL: Verify that the public key in the request matches the expected attestation public key
	// Only the HSM server with the matching private key can create valid signatures
	if !pubKey.Equal(reqPubKey) {
		return fmt.Errorf("public key does not match expected attestation public key (unauthorized commit attempt)")
	}

	// Verify signature using the expected attestation public key
	if !ecdsa.VerifyASN1(pubKey, hash[:], sigBytes) {
		return fmt.Errorf("signature verification failed (invalid signature)")
	}

	return nil
}

func (f *KeyIndexFSM) verifySignature(entry *KeyIndexEntry) error {
	// Create data to verify (key_id + index)
	data := fmt.Sprintf("%s:%d", entry.KeyID, entry.Index)
	hash := sha256.Sum256([]byte(data))
	
	// Debug: log the data being verified (remove in production if needed)
	fmt.Printf("[DEBUG] Verifying signature for data: %s, hash: %x\n", data, hash)

	// Decode signature (base64 encoded ASN.1)
	fmt.Printf("[DEBUG] Received signature (base64): %s\n", entry.Signature)
	fmt.Printf("[DEBUG] Received public key (base64): %s\n", entry.PublicKey)
	
	sigBytes, err := base64.StdEncoding.DecodeString(entry.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %v", err)
	}
	fmt.Printf("[DEBUG] Decoded signature length: %d bytes\n", len(sigBytes))

	// Decode public key from the entry
	pubKeyBytes, err := base64.StdEncoding.DecodeString(entry.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %v", err)
	}
	fmt.Printf("[DEBUG] Decoded public key length: %d bytes\n", len(pubKeyBytes))

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

// validateHashChain validates that the previous_hash matches the stored hash from the previous entry
// We use the stored hash directly - never recompute it
func (f *KeyIndexFSM) validateHashChain(entry *KeyIndexEntry) error {
	lastHash, exists := f.keyHashes[entry.KeyID]

	if !exists {
		// First entry for this key_id: previous_hash must be genesis
		if entry.PreviousHash != GenesisHash {
			return fmt.Errorf("first entry previous_hash mismatch: expected %s (genesis), got %s",
				GenesisHash, entry.PreviousHash)
		}
		return nil
	}

	// Not the first entry: previous_hash MUST match the stored hash from the previous entry
	// We use the stored hash directly - it's the actual hash that was computed and stored when that entry was committed
	if entry.PreviousHash != lastHash {
		return fmt.Errorf("hash chain broken: expected previous_hash %s (stored from previous commit), got %s",
			lastHash, entry.PreviousHash)
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

// GetKeyHash returns the hash of the last entry for a key_id
func (f *KeyIndexFSM) GetKeyHash(keyID string) (string, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	hash, exists := f.keyHashes[keyID]
	return hash, exists
}

// GetKeyIndexAndHash returns both the last index and hash for a key_id
func (f *KeyIndexFSM) GetKeyIndexAndHash(keyID string) (uint64, string, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	index, indexExists := f.keyIndices[keyID]
	hash, hashExists := f.keyHashes[keyID]

	if !indexExists || !hashExists {
		return 0, "", false
	}

	return index, hash, true
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

// GetKeyChain returns all entries for a key_id (full hash chain)
func (f *KeyIndexFSM) GetKeyChain(keyID string) ([]*KeyIndexEntry, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, exists := f.keyEntries[keyID]
	if !exists || len(entries) == 0 {
		return nil, false
	}

	// Return copies to avoid race conditions
	result := make([]*KeyIndexEntry, len(entries))
	for i, entry := range entries {
		result[i] = &KeyIndexEntry{
			KeyID:       entry.KeyID,
			Index:       entry.Index,
			PreviousHash: entry.PreviousHash,
			Hash:        entry.Hash,
			Signature:   entry.Signature,
			PublicKey:   entry.PublicKey,
		}
	}

	return result, true
}

// Snapshot creates a snapshot
func (f *KeyIndexFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create copies to avoid race conditions
	indicesCopy := make(map[string]uint64)
	hashesCopy := make(map[string]string)
	for k, v := range f.keyIndices {
		indicesCopy[k] = v
	}
	for k, v := range f.keyHashes {
		hashesCopy[k] = v
	}

	return &keyIndexSnapshot{
		keyIndices: indicesCopy,
		keyHashes:  hashesCopy,
	}, nil
}

// Restore restores from snapshot
func (f *KeyIndexFSM) Restore(r io.ReadCloser) error {
	// For now, restore is a no-op
	return nil
}

type keyIndexSnapshot struct {
	keyIndices map[string]uint64
	keyHashes  map[string]string
}

func (s *keyIndexSnapshot) Persist(sink raft.SnapshotSink) error {
	snapshot := struct {
		KeyIndices map[string]uint64 `json:"key_indices"`
		KeyHashes  map[string]string `json:"key_hashes"`
	}{
		KeyIndices: s.keyIndices,
		KeyHashes:  s.keyHashes,
	}

	data, err := json.Marshal(snapshot)
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

