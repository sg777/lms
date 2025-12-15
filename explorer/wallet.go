package explorer

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleWalletList lists all CHIPS wallets for the authenticated user
func (s *ExplorerServer) handleWalletList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from token
	tokenString := extractTokenFromHeader(r)
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get wallets from database
	wallets, err := s.walletDB.GetWalletsByUserID(claims.UserID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get wallets: %v", err), http.StatusInternalServerError)
		return
	}

	// Update balances from blockchain
	client := newVerusClientFromEnv()

	for _, wallet := range wallets {
		balance, err := client.GetBalance(wallet.Address)
		if err == nil {
			wallet.Balance = balance
			// Update cached balance in database
			s.walletDB.UpdateWalletBalance(wallet.ID, balance)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"wallets": wallets,
	})
}

// handleWalletCreate creates a new CHIPS wallet for the authenticated user
func (s *ExplorerServer) handleWalletCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user from token
	tokenString := extractTokenFromHeader(r)
	if tokenString == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate new CHIPS address with user label for easier identification
	client := newVerusClientFromEnv()

	// Create address with user label (helps identify in Verus wallet)
	addressLabel := fmt.Sprintf("user_%s", claims.UserID)
	address, err := client.GetNewAddressWithLabel(addressLabel)
	if err != nil {
		// Fallback to address without label if label fails
		address, err = client.GetNewAddress()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to generate CHIPS address: %v", err), http.StatusInternalServerError)
			return
		}
	}

	// Create wallet record
	walletID := generateWalletID()
	wallet := &CHIPSWallet{
		ID:        walletID,
		UserID:    claims.UserID,
		Address:   address,
		Balance:   0.0,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	// Store wallet
	if err := s.walletDB.StoreWallet(wallet); err != nil {
		http.Error(w, fmt.Sprintf("Failed to store wallet: %v", err), http.StatusInternalServerError)
		return
	}

	// Get initial balance
	balance, err := client.GetBalance(address)
	if err == nil {
		wallet.Balance = balance
		s.walletDB.UpdateWalletBalance(walletID, balance)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"wallet":  wallet,
	})
}

// handleWalletBalance gets the balance for a specific wallet address
func (s *ExplorerServer) handleWalletBalance(w http.ResponseWriter, r *http.Request) {
	// Safety against panics
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("internal error: %v", rec),
			})
		}
	}()

	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Get user from token
	tokenString := extractTokenFromHeader(r)
	if tokenString == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	// Get address from query parameter
	address := r.URL.Query().Get("address")
	if address == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "address parameter is required",
		})
		return
	}

	// Verify wallet belongs to user
	wallet, err := s.walletDB.GetWalletByAddress(address)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Wallet not found",
		})
		return
	}

	if wallet.UserID != claims.UserID {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized: wallet does not belong to user",
		})
		return
	}

	// Get balance from blockchain
	client := newVerusClientFromEnv()

	balance, err := client.GetBalance(address)
	if err != nil {
		// Do not return 500; return JSON with success=false so UI can handle gracefully
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to get balance: %v", err),
			"balance": 0.0,
		})
		return
	}

	// Update cached balance
	s.walletDB.UpdateWalletBalance(wallet.ID, balance)
	wallet.Balance = balance

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"address": address,
		"balance": balance,
		"wallet":  wallet,
	})
}

// generateWalletID generates a unique wallet ID
func generateWalletID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// GetUserWalletForFunding gets a user's wallet address for funding blockchain transactions
// Returns the first wallet with sufficient balance, or the first wallet if none have balance
func (s *ExplorerServer) GetUserWalletForFunding(userID string, minBalance float64) (string, error) {
	wallets, err := s.walletDB.GetWalletsByUserID(userID)
	if err != nil {
		return "", fmt.Errorf("failed to get wallets: %v", err)
	}

	if len(wallets) == 0 {
		return "", fmt.Errorf("no CHIPS wallets found. Please create a wallet first")
	}

	// Get balances and find wallet with sufficient funds
	client := newVerusClientFromEnv()

	for _, wallet := range wallets {
		balance, err := client.GetBalance(wallet.Address)
		if err == nil {
			s.walletDB.UpdateWalletBalance(wallet.ID, balance)
			if balance >= minBalance {
				return wallet.Address, nil
			}
		}
	}

	// Return first wallet even if balance is insufficient (caller will check)
	return wallets[0].Address, nil
}

// CheckWalletBalance checks if a wallet has sufficient balance
func (s *ExplorerServer) CheckWalletBalance(address string, minBalance float64) (bool, float64, error) {
	client := newVerusClientFromEnv()

	balance, err := client.GetBalance(address)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get balance: %v", err)
	}

	return balance >= minBalance, balance, nil
}
