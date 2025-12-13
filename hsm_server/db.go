package hsm_server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"go.etcd.io/bbolt"
)

const (
	dbFileName = "hsm-data/keys.db"
	bucketName = "lms_keys"
)

// KeyDB manages persistent storage for LMS keys
type KeyDB struct {
	db   *bbolt.DB
	mu   sync.RWMutex
	path string
}

// NewKeyDB creates or opens a new key database
func NewKeyDB(dbPath string) (*KeyDB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %v", err)
	}

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create bucket if it doesn't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucketName))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %v", err)
	}

	return &KeyDB{
		db:   db,
		path: dbPath,
	}, nil
}

// StoreKey stores an LMS key in the database
func (kdb *KeyDB) StoreKey(keyID string, key *LMSKey) error {
	kdb.mu.Lock()
	defer kdb.mu.Unlock()

	data, err := json.Marshal(key)
	if err != nil {
		return fmt.Errorf("failed to marshal key: %v", err)
	}

	return kdb.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		return bucket.Put([]byte(keyID), data)
	})
}

// GetKey retrieves an LMS key from the database
func (kdb *KeyDB) GetKey(keyID string) (*LMSKey, error) {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var key *LMSKey
	err := kdb.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		data := bucket.Get([]byte(keyID))
		if data == nil {
			return fmt.Errorf("key not found: %s", keyID)
		}

		key = &LMSKey{}
		return json.Unmarshal(data, key)
	})

	return key, err
}

// ListAllKeys returns all key IDs in the database
func (kdb *KeyDB) ListAllKeys() ([]string, error) {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var keyIDs []string
	err := kdb.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		return bucket.ForEach(func(k, v []byte) error {
			keyIDs = append(keyIDs, string(k))
			return nil
		})
	})

	return keyIDs, err
}

// GetAllKeys returns all LMS keys from the database
func (kdb *KeyDB) GetAllKeys() ([]*LMSKey, error) {
	kdb.mu.RLock()
	defer kdb.mu.RUnlock()

	var keys []*LMSKey
	err := kdb.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		return bucket.ForEach(func(k, v []byte) error {
			key := &LMSKey{}
			if err := json.Unmarshal(v, key); err != nil {
				return err
			}
			keys = append(keys, key)
			return nil
		})
	})

	return keys, err
}

// UpdateKeyIndex updates the index for a key
func (kdb *KeyDB) UpdateKeyIndex(keyID string, newIndex uint64) error {
	kdb.mu.Lock()
	defer kdb.mu.Unlock()

	return kdb.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		data := bucket.Get([]byte(keyID))
		if data == nil {
			return fmt.Errorf("key not found: %s", keyID)
		}

		key := &LMSKey{}
		if err := json.Unmarshal(data, key); err != nil {
			return err
		}

		key.Index = newIndex
		updatedData, err := json.Marshal(key)
		if err != nil {
			return err
		}

		return bucket.Put([]byte(keyID), updatedData)
	})
}

// DeleteAllKeys deletes all keys from the database
func (kdb *KeyDB) DeleteAllKeys() error {
	kdb.mu.Lock()
	defer kdb.mu.Unlock()

	return kdb.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return nil // Bucket doesn't exist, nothing to delete
		}
		
		// Delete the bucket and recreate it (fastest way to clear all keys)
		if err := tx.DeleteBucket([]byte(bucketName)); err != nil {
			return fmt.Errorf("failed to delete bucket: %v", err)
		}
		
		// Recreate the bucket
		_, err := tx.CreateBucket([]byte(bucketName))
		return err
	})
}

// DeleteKey deletes a specific key from the database
func (kdb *KeyDB) DeleteKey(keyID string) error {
	kdb.mu.Lock()
	defer kdb.mu.Unlock()

	return kdb.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketName))
		if bucket == nil {
			return fmt.Errorf("bucket not found")
		}
		return bucket.Delete([]byte(keyID))
	})
}

// Close closes the database
func (kdb *KeyDB) Close() error {
	kdb.mu.Lock()
	defer kdb.mu.Unlock()
	return kdb.db.Close()
}
