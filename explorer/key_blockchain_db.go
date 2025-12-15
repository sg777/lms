package explorer

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// KeyBlockchainSetting represents blockchain enable/disable setting for a key
type KeyBlockchainSetting struct {
	UserID    string `json:"user_id"`    // Owner user ID
	KeyID     string `json:"key_id"`     // LMS key ID
	Enabled   bool   `json:"enabled"`     // Whether blockchain commits are enabled
	TxID      string `json:"txid,omitempty"` // Last blockchain transaction ID (when enabled)
	EnabledAt string `json:"enabled_at,omitempty"` // When blockchain was enabled
}

// KeyBlockchainDB manages key blockchain settings database
type KeyBlockchainDB struct {
	db *bolt.DB
}

// NewKeyBlockchainDB creates a new key blockchain settings database
func NewKeyBlockchainDB(dbPath string) (*KeyBlockchainDB, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		// Main settings bucket: key = user_id:key_id, value = KeyBlockchainSetting JSON
		_, err := tx.CreateBucketIfNotExists([]byte("key_blockchain_settings"))
		if err != nil {
			return fmt.Errorf("failed to create key_blockchain_settings bucket: %v", err)
		}

		// User keys index: user_id -> []key_id (for quick lookup of all keys for a user)
		_, err = tx.CreateBucketIfNotExists([]byte("user_key_index"))
		if err != nil {
			return fmt.Errorf("failed to create user_key_index bucket: %v", err)
		}

		return nil
	})

	if err != nil {
		db.Close()
		return nil, err
	}

	return &KeyBlockchainDB{db: db}, nil
}

// GetSettingKey returns the composite key for user_id:key_id
func (kdb *KeyBlockchainDB) GetSettingKey(userID, keyID string) string {
	return fmt.Sprintf("%s:%s", userID, keyID)
}

// GetSetting retrieves blockchain setting for a key
func (kdb *KeyBlockchainDB) GetSetting(userID, keyID string) (*KeyBlockchainSetting, error) {
	var setting *KeyBlockchainSetting
	err := kdb.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("key_blockchain_settings"))
		if bucket == nil {
			return fmt.Errorf("key_blockchain_settings bucket not found")
		}

		key := kdb.GetSettingKey(userID, keyID)
		data := bucket.Get([]byte(key))
		if data == nil {
			// Return default (disabled) setting
			setting = &KeyBlockchainSetting{
				UserID:  userID,
				KeyID:   keyID,
				Enabled: false,
			}
			return nil
		}

		setting = &KeyBlockchainSetting{}
		if err := json.Unmarshal(data, setting); err != nil {
			return fmt.Errorf("failed to unmarshal setting: %v", err)
		}

		return nil
	})

	return setting, err
}

// SetSetting stores or updates blockchain setting for a key
func (kdb *KeyBlockchainDB) SetSetting(setting *KeyBlockchainSetting) error {
	return kdb.db.Update(func(tx *bolt.Tx) error {
		settingsBucket := tx.Bucket([]byte("key_blockchain_settings"))
		if settingsBucket == nil {
			return fmt.Errorf("key_blockchain_settings bucket not found")
		}
		userKeyIndex := tx.Bucket([]byte("user_key_index"))
		if userKeyIndex == nil {
			return fmt.Errorf("user_key_index bucket not found")
		}

		// Marshal setting
		data, err := json.Marshal(setting)
		if err != nil {
			return fmt.Errorf("failed to marshal setting: %v", err)
		}

		// Store setting
		key := kdb.GetSettingKey(setting.UserID, setting.KeyID)
		if err := settingsBucket.Put([]byte(key), data); err != nil {
			return fmt.Errorf("failed to store setting: %v", err)
		}

		// Update user key index
		userKey := []byte(setting.UserID)
		existingKeysData := userKeyIndex.Get(userKey)
		var keyIDs []string
		if existingKeysData != nil {
			if err := json.Unmarshal(existingKeysData, &keyIDs); err != nil {
				keyIDs = []string{}
			}
		}

		// Add keyID if not already present
		found := false
		for _, id := range keyIDs {
			if id == setting.KeyID {
				found = true
				break
			}
		}
		if !found {
			keyIDs = append(keyIDs, setting.KeyID)
			keyIDsData, err := json.Marshal(keyIDs)
			if err != nil {
				return fmt.Errorf("failed to marshal key IDs: %v", err)
			}
			if err := userKeyIndex.Put(userKey, keyIDsData); err != nil {
				return fmt.Errorf("failed to update user key index: %v", err)
			}
		}

		return nil
	})
}

// GetSettingsForUser retrieves all blockchain settings for a user
func (kdb *KeyBlockchainDB) GetSettingsForUser(userID string) (map[string]*KeyBlockchainSetting, error) {
	settings := make(map[string]*KeyBlockchainSetting)
	err := kdb.db.View(func(tx *bolt.Tx) error {
		userKeyIndex := tx.Bucket([]byte("user_key_index"))
		if userKeyIndex == nil {
			return fmt.Errorf("user_key_index bucket not found")
		}
		settingsBucket := tx.Bucket([]byte("key_blockchain_settings"))
		if settingsBucket == nil {
			return fmt.Errorf("key_blockchain_settings bucket not found")
		}

		userKeyData := userKeyIndex.Get([]byte(userID))
		if userKeyData == nil {
			// User has no keys with settings yet
			return nil
		}

		var keyIDs []string
		if err := json.Unmarshal(userKeyData, &keyIDs); err != nil {
			return fmt.Errorf("failed to unmarshal key IDs: %v", err)
		}

		for _, keyID := range keyIDs {
			key := kdb.GetSettingKey(userID, keyID)
			data := settingsBucket.Get([]byte(key))
			if data != nil {
				setting := &KeyBlockchainSetting{}
				if err := json.Unmarshal(data, setting); err == nil {
					settings[keyID] = setting
				}
			}
		}

		return nil
	})

	return settings, err
}

// Close closes the database
func (kdb *KeyBlockchainDB) Close() error {
	return kdb.db.Close()
}

