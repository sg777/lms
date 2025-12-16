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

	// Add user_id and username to request
	reqBody["user_id"] = claims.UserID
	reqBody["username"] = claims.Username

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

	// Check if blockchain is enabled for this key FIRST
	keyID, _ := reqBody["key_id"].(string)
	blockchainEnabled := false
	if keyID != "" {
		setting, err := s.keyBlockchainDB.GetSetting(claims.UserID, keyID)
		if err == nil && setting != nil {
			blockchainEnabled = setting.Enabled
		}
	}
	reqBody["blockchain_enabled"] = blockchainEnabled

	// Only check balance if blockchain is enabled
	// If blockchain is disabled, commit only goes to Raft (no balance check needed)
	const minBalanceForTx = 0.0001
	var walletWithBalance *CHIPSWallet
	var maxBalance float64

	if blockchainEnabled {
		// Blockchain enabled: Check wallet balance before allowing sign operations
		// Sign operations commit to blockchain, so we require sufficient CHIPS balance
		
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

		// Add wallet address to request (for blockchain funding)
		reqBody["wallet_address"] = walletWithBalance.Address
		log.Printf("[INFO] User %s signing with blockchain enabled - wallet %s (balance: %.8f CHIPS)", claims.UserID, walletWithBalance.Address, maxBalance)
	} else {
		// Blockchain disabled: Commit only to Raft, no balance check needed
		log.Printf("[INFO] User %s signing with blockchain disabled - commit will go to Raft only", claims.UserID)
	}

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
	userID := claims.UserID

	// Read request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Add user_id to request
	reqBody["user_id"] = userID

	// Get blockchain enablement status for this key
	keyID, ok := reqBody["key_id"].(string)
	if ok && keyID != "" {
		setting, err := s.keyBlockchainDB.GetSetting(userID, keyID)
		if err == nil && setting != nil && setting.Enabled {
			// Blockchain is enabled for this key - get wallet address for funding
			reqBody["blockchain_enabled"] = true

			// Get user's wallets to find a funding address
			wallets, err := s.walletDB.GetWalletsByUserID(userID)
			if err == nil && len(wallets) > 0 {
				// Use first wallet with balance, or first wallet if none have balance
				for _, wallet := range wallets {
					if wallet.Address != "" {
						reqBody["wallet_address"] = wallet.Address
						break
					}
				}
			}
		} else {
			reqBody["blockchain_enabled"] = false
		}
	}

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

	// Read response to check if deletion was successful
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read response: %v", err), http.StatusInternalServerError)
		return
	}

	// If deletion was successful, clean up blockchain setting
	if resp.StatusCode == http.StatusOK {
		var deleteResp map[string]interface{}
		if err := json.Unmarshal(respBody, &deleteResp); err == nil {
			if success, ok := deleteResp["success"].(bool); ok && success {
				// Delete blockchain setting for this key
				if keyID != "" {
					if err := s.keyBlockchainDB.DeleteSetting(userID, keyID); err != nil {
						log.Printf("[WARNING] Failed to delete blockchain setting for key %s: %v", keyID, err)
						// Don't fail the request - key is already deleted
					} else {
						log.Printf("[INFO] Deleted blockchain setting for key %s", keyID)
					}
				}
			}
		}
	}

	// Copy response to client
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
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
