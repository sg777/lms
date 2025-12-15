package explorer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// handleMyKeys lists keys for the authenticated user
func (s *ExplorerServer) handleMyKeys(w http.ResponseWriter, r *http.Request) {
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

	// Forward request to HSM server with user context
	url := fmt.Sprintf("%s/list_keys?user_id=%s", s.hsmEndpoint, claims.UserID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleGenerateKey generates a key for the authenticated user
func (s *ExplorerServer) handleGenerateKey(w http.ResponseWriter, r *http.Request) {
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

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request
	reqBody["user_id"] = claims.UserID

	// Forward request to HSM server
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/generate_key", s.hsmEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleSign signs a message for the authenticated user
func (s *ExplorerServer) handleSign(w http.ResponseWriter, r *http.Request) {
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

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request for verification
	reqBody["user_id"] = claims.UserID

	// REQUIRED: Check wallet balance before allowing sign operations
	// Sign operations commit to blockchain, so we require sufficient CHIPS balance
	// Minimum balance needed: ~0.0001 CHIPS for transaction fee
	const minBalanceForTx = 0.0001

	// Get user's wallets
	wallets, err := s.walletDB.GetWalletsByUserID(claims.UserID)
	if err != nil || len(wallets) == 0 {
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   "No CHIPS wallet found. Please create a wallet in the Wallet tab before signing messages.",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired) // 402 Payment Required
		json.NewEncoder(w).Encode(errorMsg)
		return
	}

	// Check balance for each wallet - need at least one with sufficient balance
	client := newVerusClientFromEnv()

	var walletWithBalance *CHIPSWallet
	var maxBalance float64

	for _, wallet := range wallets {
		balance, err := client.GetBalance(wallet.Address)
		if err == nil {
			s.walletDB.UpdateWalletBalance(wallet.ID, balance)
			if balance >= minBalanceForTx {
				if walletWithBalance == nil || balance > maxBalance {
					walletWithBalance = wallet
					maxBalance = balance
				}
			}
		}
	}

	// If no wallet has sufficient balance, block the operation
	if walletWithBalance == nil {
		// Get the highest balance to show in error message
		var highestBalance float64
		for _, wallet := range wallets {
			balance, _ := client.GetBalance(wallet.Address)
			if balance > highestBalance {
				highestBalance = balance
			}
		}

		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Insufficient CHIPS balance. Your wallet balance: %.8f CHIPS. Minimum required: %.8f CHIPS. Please load CHIPS to your wallet before signing.", highestBalance, minBalanceForTx),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired) // 402 Payment Required
		json.NewEncoder(w).Encode(errorMsg)
		return
	}

	// Add wallet address to request (for reference - Verus RPC uses wallet funds automatically)
	reqBody["wallet_address"] = walletWithBalance.Address

	// Check if blockchain is enabled for this key
	keyID, _ := reqBody["key_id"].(string)
	blockchainEnabled := false
	if keyID != "" {
		setting, err := s.keyBlockchainDB.GetSetting(claims.UserID, keyID)
		if err == nil && setting != nil {
			blockchainEnabled = setting.Enabled
		}
	}
	reqBody["blockchain_enabled"] = blockchainEnabled

	log.Printf("[INFO] User %s signing with wallet %s (balance: %.8f CHIPS, blockchain: %v)", claims.UserID, walletWithBalance.Address, maxBalance, blockchainEnabled)

	// Forward request to HSM server
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/sign", s.hsmEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleExportKey exports a key for the authenticated user
func (s *ExplorerServer) handleExportKey(w http.ResponseWriter, r *http.Request) {
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

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request
	reqBody["user_id"] = claims.UserID

	// Forward request to HSM server
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/export_key", s.hsmEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleImportKey imports a key for the authenticated user
func (s *ExplorerServer) handleImportKey(w http.ResponseWriter, r *http.Request) {
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

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request
	reqBody["user_id"] = claims.UserID

	// Forward request to HSM server
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/import_key", s.hsmEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleDeleteKey deletes a key for the authenticated user
func (s *ExplorerServer) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
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

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request
	reqBody["user_id"] = claims.UserID

	// Forward request to HSM server
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/delete_key", s.hsmEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// handleVerify verifies a signature for the authenticated user
func (s *ExplorerServer) handleVerify(w http.ResponseWriter, r *http.Request) {
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

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request for key ownership verification (if key_id provided)
	reqBody["user_id"] = claims.UserID

	// Forward request to HSM server
	jsonData, _ := json.Marshal(reqBody)
	url := fmt.Sprintf("%s/verify", s.hsmEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenString)

	resp, err := s.client.Do(req)
	if err != nil {
		// HSM server is not reachable - this is a critical error
		errorMsg := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("HSM server is not available at %s: %v. Please ensure HSM server is running.", s.hsmEndpoint, err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(errorMsg)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
