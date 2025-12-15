package explorer

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// CHIPSWallet represents a CHIPS wallet key for a user
type CHIPSWallet struct {
	ID        string  `json:"id"`         // Unique wallet key ID
	UserID    string  `json:"user_id"`    // Owner user ID
	Address   string  `json:"address"`    // CHIPS address (pubkey)
	Balance   float64 `json:"balance"`    // Current balance (cached, updated on query)
	CreatedAt string  `json:"created_at"` // Creation timestamp
}

// WalletDB manages wallet database
type WalletDB struct {
	db *bolt.DB
}

// NewWalletDB creates a new wallet database
func NewWalletDB(dbPath string) (*WalletDB, error) {
	db, err := bolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Create buckets
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("wallets"))
		if err != nil {
			return fmt.Errorf("failed to create wallets bucket: %v", err)
		}

		// Create user wallet index (user_id -> []wallet_id)
		_, err = tx.CreateBucketIfNotExists([]byte("user_wallets"))
		if err != nil {
			return fmt.Errorf("failed to create user_wallets bucket: %v", err)
		}

		// Create address index (address -> wallet_id)
		_, err = tx.CreateBucketIfNotExists([]byte("address_index"))
		if err != nil {
			return fmt.Errorf("failed to create address_index bucket: %v", err)
		}

		return nil
	})

	if err != nil {
		db.Close()
		return nil, err
	}

	return &WalletDB{db: db}, nil
}

// StoreWallet stores a wallet in the database
func (wdb *WalletDB) StoreWallet(wallet *CHIPSWallet) error {
	return wdb.db.Update(func(tx *bolt.Tx) error {
		walletsBucket := tx.Bucket([]byte("wallets"))
		if walletsBucket == nil {
			return fmt.Errorf("wallets bucket not found")
		}
		userWalletsBucket := tx.Bucket([]byte("user_wallets"))
		if userWalletsBucket == nil {
			return fmt.Errorf("user_wallets bucket not found")
		}
		addressIndex := tx.Bucket([]byte("address_index"))
		if addressIndex == nil {
			return fmt.Errorf("address_index bucket not found")
		}

		// Check if address already exists
		if existingID := addressIndex.Get([]byte(wallet.Address)); existingID != nil {
			if string(existingID) != wallet.ID {
				return fmt.Errorf("address already exists")
			}
		}

		// Marshal wallet
		data, err := json.Marshal(wallet)
		if err != nil {
			return fmt.Errorf("failed to marshal wallet: %v", err)
		}

		// Store wallet by ID
		if err := walletsBucket.Put([]byte(wallet.ID), data); err != nil {
			return fmt.Errorf("failed to store wallet: %v", err)
		}

		// Update address index
		if err := addressIndex.Put([]byte(wallet.Address), []byte(wallet.ID)); err != nil {
			return fmt.Errorf("failed to update address index: %v", err)
		}

		// Update user wallets index
		userWalletsKey := []byte(wallet.UserID)
		existingWallets := userWalletsBucket.Get(userWalletsKey)
		var walletIDs []string
		if existingWallets != nil {
			if err := json.Unmarshal(existingWallets, &walletIDs); err != nil {
				walletIDs = []string{}
			}
		}
		// Add wallet ID if not already present
		found := false
		for _, id := range walletIDs {
			if id == wallet.ID {
				found = true
				break
			}
		}
		if !found {
			walletIDs = append(walletIDs, wallet.ID)
			walletIDsData, err := json.Marshal(walletIDs)
			if err != nil {
				return fmt.Errorf("failed to marshal wallet IDs: %v", err)
			}
			if err := userWalletsBucket.Put(userWalletsKey, walletIDsData); err != nil {
				return fmt.Errorf("failed to update user wallets index: %v", err)
			}
		}

		return nil
	})
}

// GetWalletByID retrieves a wallet by ID
func (wdb *WalletDB) GetWalletByID(walletID string) (*CHIPSWallet, error) {
	var wallet *CHIPSWallet
	err := wdb.db.View(func(tx *bolt.Tx) error {
		walletsBucket := tx.Bucket([]byte("wallets"))
		if walletsBucket == nil {
			return fmt.Errorf("wallets bucket not found")
		}
		data := walletsBucket.Get([]byte(walletID))
		if data == nil {
			return fmt.Errorf("wallet not found")
		}

		wallet = &CHIPSWallet{}
		if err := json.Unmarshal(data, wallet); err != nil {
			return fmt.Errorf("failed to unmarshal wallet: %v", err)
		}

		return nil
	})

	return wallet, err
}

// GetWalletsByUserID retrieves all wallets for a user
func (wdb *WalletDB) GetWalletsByUserID(userID string) ([]*CHIPSWallet, error) {
	var wallets []*CHIPSWallet
	err := wdb.db.View(func(tx *bolt.Tx) error {
		userWalletsBucket := tx.Bucket([]byte("user_wallets"))
		if userWalletsBucket == nil {
			return fmt.Errorf("user_wallets bucket not found")
		}
		walletsBucket := tx.Bucket([]byte("wallets"))
		if walletsBucket == nil {
			return fmt.Errorf("wallets bucket not found")
		}

		userWalletsData := userWalletsBucket.Get([]byte(userID))
		if userWalletsData == nil {
			// User has no wallets
			return nil
		}

		var walletIDs []string
		if err := json.Unmarshal(userWalletsData, &walletIDs); err != nil {
			return fmt.Errorf("failed to unmarshal wallet IDs: %v", err)
		}

		for _, walletID := range walletIDs {
			walletData := walletsBucket.Get([]byte(walletID))
			if walletData != nil {
				wallet := &CHIPSWallet{}
				if err := json.Unmarshal(walletData, wallet); err == nil {
					wallets = append(wallets, wallet)
				}
			}
		}

		return nil
	})

	return wallets, err
}

// GetWalletByAddress retrieves a wallet by address
func (wdb *WalletDB) GetWalletByAddress(address string) (*CHIPSWallet, error) {
	var wallet *CHIPSWallet
	err := wdb.db.View(func(tx *bolt.Tx) error {
		addressIndex := tx.Bucket([]byte("address_index"))
		if addressIndex == nil {
			return fmt.Errorf("address_index bucket not found")
		}
		walletID := addressIndex.Get([]byte(address))
		if walletID == nil {
			return fmt.Errorf("wallet not found for address")
		}

		walletsBucket := tx.Bucket([]byte("wallets"))
		if walletsBucket == nil {
			return fmt.Errorf("wallets bucket not found")
		}
		data := walletsBucket.Get(walletID)
		if data == nil {
			return fmt.Errorf("wallet not found")
		}

		wallet = &CHIPSWallet{}
		if err := json.Unmarshal(data, wallet); err != nil {
			return fmt.Errorf("failed to unmarshal wallet: %v", err)
		}

		return nil
	})

	return wallet, err
}

// UpdateWalletBalance updates the cached balance for a wallet
func (wdb *WalletDB) UpdateWalletBalance(walletID string, balance float64) error {
	return wdb.db.Update(func(tx *bolt.Tx) error {
		walletsBucket := tx.Bucket([]byte("wallets"))
		if walletsBucket == nil {
			return fmt.Errorf("wallets bucket not found")
		}

		data := walletsBucket.Get([]byte(walletID))
		if data == nil {
			return fmt.Errorf("wallet not found")
		}

		wallet := &CHIPSWallet{}
		if err := json.Unmarshal(data, wallet); err != nil {
			return fmt.Errorf("failed to unmarshal wallet: %v", err)
		}

		wallet.Balance = balance

		updatedData, err := json.Marshal(wallet)
		if err != nil {
			return fmt.Errorf("failed to marshal wallet: %v", err)
		}

		return walletsBucket.Put([]byte(walletID), updatedData)
	})
}

// Close closes the database
func (wdb *WalletDB) Close() error {
	return wdb.db.Close()
}

