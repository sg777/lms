package explorer

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// UserDB manages user database
type UserDB struct {
	db *bolt.DB
}

// NewUserDB creates a new user database
func NewUserDB(dbPath string) (*UserDB, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("users"))
		if err != nil {
			return fmt.Errorf("failed to create users bucket: %v", err)
		}
		
		// Create username index bucket
		_, err = tx.CreateBucketIfNotExists([]byte("username_index"))
		if err != nil {
			return fmt.Errorf("failed to create username_index bucket: %v", err)
		}
		
		return nil
	})

	if err != nil {
		db.Close()
		return nil, err
	}

	return &UserDB{db: db}, nil
}

// StoreUser stores a user in the database
func (udb *UserDB) StoreUser(user *User) error {
	return udb.db.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte("users"))
		if usersBucket == nil {
			return fmt.Errorf("users bucket not found")
		}
		usernameIndex := tx.Bucket([]byte("username_index"))
		if usernameIndex == nil {
			return fmt.Errorf("username_index bucket not found")
		}

		// Check if username already exists
		if existingID := usernameIndex.Get([]byte(user.Username)); existingID != nil {
			if string(existingID) != user.ID {
				return fmt.Errorf("username already exists")
			}
		}

		// Marshal user
		data, err := json.Marshal(user)
		if err != nil {
			return fmt.Errorf("failed to marshal user: %v", err)
		}

		// Store user by ID
		if err := usersBucket.Put([]byte(user.ID), data); err != nil {
			return fmt.Errorf("failed to store user: %v", err)
		}

		// Update username index
		if err := usernameIndex.Put([]byte(user.Username), []byte(user.ID)); err != nil {
			return fmt.Errorf("failed to update username index: %v", err)
		}

		return nil
	})
}

// GetUserByID retrieves a user by ID
func (udb *UserDB) GetUserByID(userID string) (*User, error) {
	var user *User
	err := udb.db.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte("users"))
		data := usersBucket.Get([]byte(userID))
		if data == nil {
			return fmt.Errorf("user not found")
		}

		user = &User{}
		if err := json.Unmarshal(data, user); err != nil {
			return fmt.Errorf("failed to unmarshal user: %v", err)
		}

		return nil
	})

	return user, err
}

// GetUserByUsername retrieves a user by username
func (udb *UserDB) GetUserByUsername(username string) (*User, error) {
	var user *User
	err := udb.db.View(func(tx *bolt.Tx) error {
		usernameIndex := tx.Bucket([]byte("username_index"))
		if usernameIndex == nil {
			return fmt.Errorf("username_index bucket not found")
		}
		userID := usernameIndex.Get([]byte(username))
		if userID == nil {
			// Debug: list all usernames in index
			c := usernameIndex.Cursor()
			keys := []string{}
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				keys = append(keys, string(k))
			}
			return fmt.Errorf("user not found (looking for '%s', available: %v)", username, keys)
		}

		usersBucket := tx.Bucket([]byte("users"))
		if usersBucket == nil {
			return fmt.Errorf("users bucket not found")
		}
		data := usersBucket.Get(userID)
		if data == nil {
			return fmt.Errorf("user not found")
		}

		user = &User{}
		if err := json.Unmarshal(data, user); err != nil {
			return fmt.Errorf("failed to unmarshal user: %v", err)
		}

		return nil
	})

	return user, err
}

// Close closes the database
func (udb *UserDB) Close() error {
	return udb.db.Close()
}

