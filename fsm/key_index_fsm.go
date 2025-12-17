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
	"sort"
	"sync"

	"github.com/hashicorp/raft"
)

// KeyIndexEntry represents an index commitment for a pubkey_hash with hash chain
type KeyIndexEntry struct {
	KeyID        string `json:"key_id"`      // User-friendly label (for display, can change)
	PubkeyHash   string `json:"pubkey_hash"` // SHA-256 hash of LMS public key (primary identifier)
	Index        uint64 `json:"index"`
	PreviousHash string `json:"previous_hash"` // SHA-256 hash of the previous entry (genesis: all 0's)
	Hash         string `json:"hash"`          // SHA-256 hash of this entry (computed on all fields except hash)
	Signature    string `json:"signature"`     // Base64 encoded EC signature
	PublicKey    string `json:"public_key"`    // Base64 encoded EC public key (for verification)
	RecordType   string `json:"record_type"`   // Record type: "create", "sign", "sync", "delete"
}

// ComputeHash computes the SHA-256 hash of the entry
// Hash is computed on all fields EXCEPT the Hash field itself
func (e *KeyIndexEntry) ComputeHash() (string, error) {
	// Create a temporary entry without the Hash field for computing hash
	tempEntry := struct {
		KeyID        string `json:"key_id"`
		PubkeyHash   string `json:"pubkey_hash"`
		Index        uint64 `json:"index"`
		PreviousHash string `json:"previous_hash"`
		Signature    string `json:"signature"`
		PublicKey    string `json:"public_key"`
		RecordType   string `json:"record_type"`
	}{
		KeyID:        e.KeyID,
		PubkeyHash:   e.PubkeyHash,
		Index:        e.Index,
		PreviousHash: e.PreviousHash,
		Signature:    e.Signature,
		PublicKey:    e.PublicKey,
		RecordType:   e.RecordType,
	}

	jsonData, err := json.Marshal(tempEntry)
	if err != nil {
		return "", fmt.Errorf("failed to marshal entry for hashing: %v", err)
	}

	hash := sha256.Sum256(jsonData)
	return base64.StdEncoding.EncodeToString(hash[:]), nil
}

// ComputePubkeyHash computes SHA-256 hash of the LMS public key
func ComputePubkeyHash(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// GenesisHash is the hash used for the first entry (all zeros)
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// KeyIndexFSM stores pubkey_hash -> index mappings with EC signature verification and hash chain
type KeyIndexFSM struct {
	mu                sync.RWMutex
	pubkeyHashIndices map[string]uint64           // pubkey_hash -> last used index
	pubkeyHashHashes  map[string]string           // pubkey_hash -> hash of last entry (for hash chain validation)
	pubkeyHashEntries map[string][]*KeyIndexEntry // pubkey_hash -> all entries (for full chain retrieval)
	keyIdToPubkeyHash map[string]string           // key_id -> pubkey_hash (for lookup convenience, latest mapping)
	attestationPubKey *ecdsa.PublicKey            // Public key for verifying signatures
	entryToRaftIndex  map[string]uint64           // entry hash -> Raft log index (for chronological ordering)
}

// NewKeyIndexFSM creates a new key index FSM
// attestationPubKeyPath: Path to the attestation public key PEM file
func NewKeyIndexFSM(attestationPubKeyPath string) (*KeyIndexFSM, error) {
	fsm := &KeyIndexFSM{
		pubkeyHashIndices: make(map[string]uint64),
		pubkeyHashHashes:  make(map[string]string),
		pubkeyHashEntries: make(map[string][]*KeyIndexEntry),
		keyIdToPubkeyHash: make(map[string]string),
		entryToRaftIndex:  make(map[string]uint64),
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

	// For genesis entries (index 0 with genesis previous_hash), skip hash mismatch check
	// Genesis entries are always valid - the hash may not match computed hash but that's acceptable for genesis
	if entry.Index == 0 && entry.PreviousHash == GenesisHash {
		// Genesis entry - accept it as valid, use the computed hash going forward
		entry.Hash = computedHash
	} else if entry.Hash != computedHash {
		// For non-genesis entries, hash must match
		return fmt.Sprintf("Error: Hash mismatch: expected %s, got %s", computedHash, entry.Hash)
	}

	// Validate that pubkey_hash is present (required for Phase B)
	if entry.PubkeyHash == "" {
		return fmt.Sprintf("Error: pubkey_hash is required but missing in entry")
	}

	// Use pubkey_hash as the primary identifier for lookups
	// key_id is kept for display/reference purposes only
	pubkeyHash := entry.PubkeyHash

	// Check if index is valid (must be > current index for this pubkey_hash)
	currentIndex, exists := f.pubkeyHashIndices[pubkeyHash]
	if exists && entry.Index <= currentIndex {
		return fmt.Sprintf("Error: Index %d is not greater than current index %d for pubkey_hash %s",
			entry.Index, currentIndex, pubkeyHash)
	}

	// Store the index and hash using pubkey_hash (the hash is the actual hash of this commit, computed above)
	// This stored hash will be used as previous_hash for the next entry - never recomputed
	f.pubkeyHashIndices[pubkeyHash] = entry.Index
	f.pubkeyHashHashes[pubkeyHash] = entry.Hash

	// Store key_id -> pubkey_hash mapping for lookup convenience (latest mapping)
	f.keyIdToPubkeyHash[entry.KeyID] = pubkeyHash

	// Store the full entry for chain retrieval (using pubkey_hash)
	entryCopy := &KeyIndexEntry{
		KeyID:        entry.KeyID,
		PubkeyHash:   entry.PubkeyHash,
		Index:        entry.Index,
		PreviousHash: entry.PreviousHash,
		Hash:         entry.Hash,
		Signature:    entry.Signature,
		PublicKey:    entry.PublicKey,
		RecordType:   entry.RecordType, // Include record type for proper chain retrieval
	}
	f.pubkeyHashEntries[pubkeyHash] = append(f.pubkeyHashEntries[pubkeyHash], entryCopy)
	
	// Store Raft log index for chronological ordering
	f.entryToRaftIndex[entry.Hash] = l.Index

	return fmt.Sprintf("Applied key index: key_id=%s, pubkey_hash=%s, index=%d, hash=%s", entry.KeyID, pubkeyHash, entry.Index, entry.Hash)
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
	if entry.PubkeyHash == "" {
		return fmt.Errorf("pubkey_hash is required for hash chain validation")
	}

	lastHash, exists := f.pubkeyHashHashes[entry.PubkeyHash]

	if !exists {
		// First entry for this pubkey_hash: previous_hash must be genesis
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

// GetKeyIndex returns the last used index for a key_id (looks up via pubkey_hash)
// DEPRECATED: Use GetIndexByPubkeyHash or GetIndexByKeyID instead
func (f *KeyIndexFSM) GetKeyIndex(keyID string) (uint64, bool) {
	// Try to find pubkey_hash from key_id mapping
	pubkeyHash, exists := f.keyIdToPubkeyHash[keyID]
	if !exists {
		return 0, false
	}
	return f.GetIndexByPubkeyHash(pubkeyHash)
}

// GetIndexByPubkeyHash returns the last used index for a pubkey_hash
func (f *KeyIndexFSM) GetIndexByPubkeyHash(pubkeyHash string) (uint64, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	index, exists := f.pubkeyHashIndices[pubkeyHash]
	return index, exists
}

// GetHashByPubkeyHash returns the hash of the last entry for a pubkey_hash
func (f *KeyIndexFSM) GetHashByPubkeyHash(pubkeyHash string) (string, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	hash, exists := f.pubkeyHashHashes[pubkeyHash]
	return hash, exists
}

// GetKeyHash returns the hash of the last entry for a key_id (looks up via pubkey_hash)
// DEPRECATED: Use GetHashByPubkeyHash instead
func (f *KeyIndexFSM) GetKeyHash(keyID string) (string, bool) {
	pubkeyHash, exists := f.keyIdToPubkeyHash[keyID]
	if !exists {
		return "", false
	}
	return f.GetHashByPubkeyHash(pubkeyHash)
}

// GetIndexAndHashByPubkeyHash returns both the last index and hash for a pubkey_hash
func (f *KeyIndexFSM) GetIndexAndHashByPubkeyHash(pubkeyHash string) (uint64, string, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	index, indexExists := f.pubkeyHashIndices[pubkeyHash]
	hash, hashExists := f.pubkeyHashHashes[pubkeyHash]

	if !indexExists || !hashExists {
		return 0, "", false
	}

	return index, hash, true
}

// GetKeyIndexAndHash returns both the last index and hash for a key_id (looks up via pubkey_hash)
// DEPRECATED: Use GetIndexAndHashByPubkeyHash instead
func (f *KeyIndexFSM) GetKeyIndexAndHash(keyID string) (uint64, string, bool) {
	pubkeyHash, exists := f.keyIdToPubkeyHash[keyID]
	if !exists {
		return 0, "", false
	}
	return f.GetIndexAndHashByPubkeyHash(pubkeyHash)
}

// GetAllKeyIndices returns all pubkey_hash -> index mappings
func (f *KeyIndexFSM) GetAllKeyIndices() map[string]uint64 {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make(map[string]uint64)
	for k, v := range f.pubkeyHashIndices {
		result[k] = v
	}
	return result
}

// GetAllKeyIDs returns all key_id values (for backward compatibility in explorer)
func (f *KeyIndexFSM) GetAllKeyIDs() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	result := make([]string, 0, len(f.keyIdToPubkeyHash))
	for keyID := range f.keyIdToPubkeyHash {
		result = append(result, keyID)
	}
	return result
}

// GetChainByPubkeyHash returns all entries for a pubkey_hash (full hash chain)
func (f *KeyIndexFSM) GetChainByPubkeyHash(pubkeyHash string) ([]*KeyIndexEntry, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	entries, exists := f.pubkeyHashEntries[pubkeyHash]
	if !exists || len(entries) == 0 {
		return nil, false
	}

	// Return copies to avoid race conditions
	result := make([]*KeyIndexEntry, len(entries))
	for i, entry := range entries {
		result[i] = &KeyIndexEntry{
			KeyID:        entry.KeyID,
			PubkeyHash:   entry.PubkeyHash,
			Index:        entry.Index,
			PreviousHash: entry.PreviousHash,
			Hash:         entry.Hash,
			Signature:    entry.Signature,
			PublicKey:    entry.PublicKey,
		}
	}

	return result, true
}

// GetKeyChain returns all entries for a key_id (looks up via pubkey_hash)
// DEPRECATED: Use GetAllEntriesByKeyID instead to get all entries for a key_id across all pubkey_hashes
func (f *KeyIndexFSM) GetKeyChain(keyID string) ([]*KeyIndexEntry, bool) {
	return f.GetAllEntriesByKeyID(keyID)
}

// GetAllPubkeyHashesByKeyID returns all pubkey_hashes that have entries with the given key_id
func (f *KeyIndexFSM) GetAllPubkeyHashesByKeyID(keyID string) []string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	pubkeyHashes := make(map[string]bool)

	// Iterate through all pubkey_hash entries to find all pubkey_hashes with matching key_id
	for pubkeyHash, entries := range f.pubkeyHashEntries {
		for _, entry := range entries {
			if entry.KeyID == keyID {
				pubkeyHashes[pubkeyHash] = true
				break // Found at least one entry for this pubkey_hash
			}
		}
	}

	result := make([]string, 0, len(pubkeyHashes))
	for ph := range pubkeyHashes {
		result = append(result, ph)
	}

	return result
}

// GetAllEntriesByKeyID returns ALL entries for a key_id across ALL pubkey_hashes
// This is needed when a key_id is reused (e.g., after delete and recreate)
// Entries are sorted by pubkey_hash, then by index (ascending)
func (f *KeyIndexFSM) GetAllEntriesByKeyID(keyID string) ([]*KeyIndexEntry, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	allEntries := make([]*KeyIndexEntry, 0)

	// Iterate through all pubkey_hash entries to find all entries with matching key_id
	for _, entries := range f.pubkeyHashEntries {
		for _, entry := range entries {
			if entry.KeyID == keyID {
				// Create a copy
				entryCopy := &KeyIndexEntry{
					KeyID:        entry.KeyID,
					PubkeyHash:   entry.PubkeyHash,
					Index:        entry.Index,
					PreviousHash: entry.PreviousHash,
					Hash:         entry.Hash,
					Signature:    entry.Signature,
					PublicKey:    entry.PublicKey,
					RecordType:   entry.RecordType,
				}
				allEntries = append(allEntries, entryCopy)
			}
		}
	}

	if len(allEntries) == 0 {
		return nil, false
	}

	// Sort entries: first by pubkey_hash (to group chains), then by index (ascending)
	// This ensures entries from the same chain stay together
	sort.Slice(allEntries, func(i, j int) bool {
		if allEntries[i].PubkeyHash != allEntries[j].PubkeyHash {
			return allEntries[i].PubkeyHash < allEntries[j].PubkeyHash
		}
		return allEntries[i].Index < allEntries[j].Index
	})

	return allEntries, true
}

// GetAllEntries returns ALL entries from all pubkey_hashes, ordered by Raft log index (newest first)
// Returns entries with their Raft log indices for chronological ordering
func (f *KeyIndexFSM) GetAllEntries(limit int) []struct {
	Entry     *KeyIndexEntry
	RaftIndex uint64
} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	allEntriesWithIndex := make([]struct {
		Entry     *KeyIndexEntry
		RaftIndex uint64
	}, 0)

	// Collect all entries from all pubkey_hashes
	for _, entries := range f.pubkeyHashEntries {
		for _, entry := range entries {
			raftIndex, hasIndex := f.entryToRaftIndex[entry.Hash]
			if !hasIndex {
				// If Raft index not found, skip (shouldn't happen, but be safe)
				continue
			}

			// Create a copy
			entryCopy := &KeyIndexEntry{
				KeyID:        entry.KeyID,
				PubkeyHash:   entry.PubkeyHash,
				Index:        entry.Index,
				PreviousHash: entry.PreviousHash,
				Hash:         entry.Hash,
				Signature:    entry.Signature,
				PublicKey:    entry.PublicKey,
				RecordType:   entry.RecordType,
			}

			allEntriesWithIndex = append(allEntriesWithIndex, struct {
				Entry     *KeyIndexEntry
				RaftIndex uint64
			}{
				Entry:     entryCopy,
				RaftIndex: raftIndex,
			})
		}
	}

	// Sort by Raft log index (descending - newest first)
	sort.Slice(allEntriesWithIndex, func(i, j int) bool {
		return allEntriesWithIndex[i].RaftIndex > allEntriesWithIndex[j].RaftIndex
	})

	// Apply limit
	if limit > 0 && limit < len(allEntriesWithIndex) {
		return allEntriesWithIndex[:limit]
	}

	return allEntriesWithIndex
}

// Snapshot creates a snapshot
func (f *KeyIndexFSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create copies to avoid race conditions
	indicesCopy := make(map[string]uint64)
	hashesCopy := make(map[string]string)
	keyIdMappingCopy := make(map[string]string)

	for k, v := range f.pubkeyHashIndices {
		indicesCopy[k] = v
	}
	for k, v := range f.pubkeyHashHashes {
		hashesCopy[k] = v
	}
	for k, v := range f.keyIdToPubkeyHash {
		keyIdMappingCopy[k] = v
	}

	return &keyIndexSnapshot{
		pubkeyHashIndices: indicesCopy,
		pubkeyHashHashes:  hashesCopy,
		keyIdToPubkeyHash: keyIdMappingCopy,
	}, nil
}

// Restore restores from snapshot
func (f *KeyIndexFSM) Restore(r io.ReadCloser) error {
	// For now, restore is a no-op
	return nil
}

type keyIndexSnapshot struct {
	pubkeyHashIndices map[string]uint64
	pubkeyHashHashes  map[string]string
	keyIdToPubkeyHash map[string]string
}

func (s *keyIndexSnapshot) Persist(sink raft.SnapshotSink) error {
	snapshot := struct {
		PubkeyHashIndices map[string]uint64 `json:"pubkey_hash_indices"`
		PubkeyHashHashes  map[string]string `json:"pubkey_hash_hashes"`
		KeyIdToPubkeyHash map[string]string `json:"key_id_to_pubkey_hash"`
	}{
		PubkeyHashIndices: s.pubkeyHashIndices,
		PubkeyHashHashes:  s.pubkeyHashHashes,
		KeyIdToPubkeyHash: s.keyIdToPubkeyHash,
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
