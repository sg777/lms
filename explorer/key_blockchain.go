package explorer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// handleKeyBlockchainToggle handles enabling/disabling blockchain for a key
func (s *ExplorerServer) handleKeyBlockchainToggle(w http.ResponseWriter, r *http.Request) {
	log.Printf("[BLOCKCHAIN_TOGGLE] Request received - Method: %s, URL: %s", r.Method, r.URL.String())
	
	if r.Method != http.MethodPost {
		log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Method not allowed - got %s, expected POST", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Get user from JWT token
	tokenString := extractTokenFromHeader(r)
	if tokenString == "" {
		log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: No token in request header")
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
		log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Token validation failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}
	userID := claims.UserID
	log.Printf("[BLOCKCHAIN_TOGGLE] User authenticated: userID=%s", userID)

	var req struct {
		KeyID  string `json:"key_id"`
		Enable bool   `json:"enable"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Failed to decode request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	if req.KeyID == "" {
		log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: key_id is required")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "key_id is required",
		})
		return
	}
	
	log.Printf("[BLOCKCHAIN_TOGGLE] Request: key_id=%s, enable=%v", req.KeyID, req.Enable)

	// If enabling, check wallet balance and commit current index to blockchain
	if req.Enable {
		log.Printf("[BLOCKCHAIN_TOGGLE] Enabling blockchain for key_id=%s, userID=%s", req.KeyID, userID)
		
		// Get user's wallets
		wallets, err := s.walletDB.GetWalletsByUserID(userID)
		if err != nil {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Failed to get wallets for userID=%s: %v", userID, err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "No wallet found. Please create a wallet first.",
			})
			return
		}
		if len(wallets) == 0 {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: No wallets found for userID=%s", userID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "No wallet found. Please create a wallet first.",
			})
			return
		}
		log.Printf("[BLOCKCHAIN_TOGGLE] Found %d wallets for userID=%s", len(wallets), userID)

		// Get total balance
		verusClient := newVerusClientFromEnv()

		totalBalance := 0.0
		var fundingAddress string
		for _, wallet := range wallets {
			balance, err := verusClient.GetBalance(wallet.Address)
			if err != nil {
				log.Printf("[BLOCKCHAIN_TOGGLE] WARNING: Failed to get balance for wallet %s: %v", wallet.Address, err)
			} else {
				log.Printf("[BLOCKCHAIN_TOGGLE] Wallet %s balance: %.8f CHIPS", wallet.Address, balance)
				totalBalance += balance
				if fundingAddress == "" && balance > 0.0005 {
					fundingAddress = wallet.Address // Use first wallet with balance
					log.Printf("[BLOCKCHAIN_TOGGLE] Selected funding address: %s (balance: %.8f CHIPS)", fundingAddress, balance)
				}
			}
		}
		log.Printf("[BLOCKCHAIN_TOGGLE] Total balance across all wallets: %.8f CHIPS", totalBalance)

		// Minimum balance required: 0.0005 CHIPS (actual fees are typically 0.0004 CHIPS or higher)
		minBalanceRequired := 0.0005
		if totalBalance < minBalanceRequired {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Insufficient balance - total=%.8f CHIPS, required=%.8f CHIPS", totalBalance, minBalanceRequired)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Insufficient balance: %.8f CHIPS. Need at least %.8f CHIPS for transaction fees (actual fees are typically 0.0004 CHIPS or higher).", totalBalance, minBalanceRequired),
			})
			return
		}

		if fundingAddress == "" {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: No wallet with sufficient balance found (totalBalance=%.8f but no wallet > %.8f)", totalBalance, minBalanceRequired)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "No wallet with sufficient balance found. Please fund your wallet.",
			})
			return
		}

		// CRITICAL: Check Raft availability before enabling blockchain
		// Raft must be available to enable blockchain (for consistency)
		log.Printf("[BLOCKCHAIN_TOGGLE] Querying Raft for key_id=%s to get latest index and pubkey_hash", req.KeyID)
		latestIndex, pubkeyHash, hasData, err := s.getLatestIndexFromRaft(req.KeyID)
		if err != nil {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Raft cluster unavailable for key_id=%s: %v", req.KeyID, err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Cannot enable blockchain: Raft cluster is unavailable. Error: %v. Please ensure Raft cluster is running.", err),
			})
			return
		}
		log.Printf("[BLOCKCHAIN_TOGGLE] Raft query result: hasData=%v, latestIndex=%d, pubkeyHash=%s", hasData, latestIndex, pubkeyHash)

		// If Raft has no data (nothing signed yet), just enable the setting (no blockchain commit)
		if !hasData {
			log.Printf("[BLOCKCHAIN_TOGGLE] No Raft data for key_id=%s, enabling setting without blockchain commit", req.KeyID)
			// Store setting without committing to blockchain
			setting := &KeyBlockchainSetting{
				UserID:    userID,
				KeyID:     req.KeyID,
				Enabled:   true,
				EnabledAt: time.Now().Format(time.RFC3339),
			}

			if err := s.keyBlockchainDB.SetSetting(setting); err != nil {
				log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Failed to save setting for key_id=%s: %v", req.KeyID, err)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("Failed to save setting: %v", err),
				})
				return
			}

			log.Printf("[BLOCKCHAIN_TOGGLE] Successfully enabled blockchain setting for key_id=%s (no Raft data, no commit)", req.KeyID)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"enabled": true,
				"message": "Blockchain enabled! No data in Raft yet - setting enabled. Future commits will go to both Raft and blockchain.",
			})
			return
		}

		// Raft has data: Commit current latest index to blockchain
		identityName := verusIdentityName()
		log.Printf("[BLOCKCHAIN_TOGGLE] Committing to blockchain: identityName=%s, key_id=%s, pubkeyHash=%s, index=%d, fundingAddress=%s", 
			identityName, req.KeyID, pubkeyHash, latestIndex, fundingAddress)
		
		normalizedKeyID, txID, err := verusClient.CommitLMSIndexWithPubkeyHash(
			identityName,
			pubkeyHash,
			fmt.Sprintf("%d", latestIndex),
			fundingAddress,
		)

		if err != nil {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Failed to commit to blockchain - key_id=%s, pubkeyHash=%s, index=%d, fundingAddress=%s, identityName=%s, error=%v", 
				req.KeyID, pubkeyHash, latestIndex, fundingAddress, identityName, err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Failed to commit to blockchain: %v", err),
			})
			return
		}
		
		log.Printf("[BLOCKCHAIN_TOGGLE] Successfully committed to blockchain: key_id=%s, normalizedKeyID=%s, txID=%s, index=%d", 
			req.KeyID, normalizedKeyID, txID, latestIndex)

		// Store setting
		setting := &KeyBlockchainSetting{
			UserID:    userID,
			KeyID:     req.KeyID,
			Enabled:   true,
			TxID:      txID,
			EnabledAt: time.Now().Format(time.RFC3339),
		}

		if err := s.keyBlockchainDB.SetSetting(setting); err != nil {
			log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Failed to save setting after successful commit - key_id=%s, txID=%s, error=%v", req.KeyID, txID, err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Failed to save setting: %v", err),
			})
			return
		}

		log.Printf("[BLOCKCHAIN_TOGGLE] SUCCESS: Blockchain enabled for key_id=%s, txID=%s, normalizedKeyID=%s, index=%d", 
			req.KeyID, txID, normalizedKeyID, latestIndex)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":           true,
			"enabled":           true,
			"txid":              txID,
			"normalized_key_id": normalizedKeyID,
			"index_committed":   latestIndex,
			"message":           fmt.Sprintf("Blockchain enabled! Index %d committed to blockchain (tx: %s)", latestIndex, txID),
		})
		return
	}

	// Disabling blockchain
	log.Printf("[BLOCKCHAIN_TOGGLE] Disabling blockchain for key_id=%s, userID=%s", req.KeyID, userID)
	setting := &KeyBlockchainSetting{
		UserID:  userID,
		KeyID:   req.KeyID,
		Enabled: false,
	}

	if err := s.keyBlockchainDB.SetSetting(setting); err != nil {
		log.Printf("[BLOCKCHAIN_TOGGLE] ERROR: Failed to save setting (disable) - key_id=%s, error=%v", req.KeyID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to save setting: %v", err),
		})
		return
	}

	log.Printf("[BLOCKCHAIN_TOGGLE] SUCCESS: Blockchain disabled for key_id=%s", req.KeyID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"enabled": false,
		"message": "Blockchain disabled for this key",
	})
}

// handleKeyBlockchainStatus returns blockchain status for all user's keys
func (s *ExplorerServer) handleKeyBlockchainStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("[BLOCKCHAIN_STATUS] Handler called - Method: %s, URL: %s", r.Method, r.URL.String())
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Method not allowed",
		})
		return
	}

	// Get user from JWT token
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
	userID := claims.UserID

	settings, err := s.keyBlockchainDB.GetSettingsForUser(userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to get settings: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"settings": settings,
	})
}

// getLatestIndexFromRaft gets the latest index and pubkey_hash for a key from Raft
// Returns: (index, pubkeyHash, hasData, error)
// hasData indicates if the key has any commits in Raft (false if key doesn't exist or has no data)
func (s *ExplorerServer) getLatestIndexFromRaft(keyID string) (uint64, string, bool, error) {
	var lastErr error
	
	// Query Raft for the key's chain
	for _, endpoint := range s.raftEndpoints {
		url := fmt.Sprintf("%s/key/%s/chain", endpoint, keyID)
		resp, err := http.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			var chainResp map[string]interface{}
			if err := json.Unmarshal(body, &chainResp); err == nil {
				if success, _ := chainResp["success"].(bool); success {
					if chain, ok := chainResp["chain"].([]interface{}); ok && len(chain) > 0 {
						// Get the last entry (highest index)
						lastEntry := chain[len(chain)-1]
						if entryMap, ok := lastEntry.(map[string]interface{}); ok {
							// Get index
							var index uint64
							if idx, ok := entryMap["index"].(float64); ok {
								index = uint64(idx)
							}

							// Get pubkey_hash
							pubkeyHash := ""
							if ph, ok := entryMap["pubkey_hash"].(string); ok && ph != "" {
								// pubkey_hash from Raft is stored in base64 format
								// Convert to hex for blockchain operations
								pubkeyHashBytes, err := base64.StdEncoding.DecodeString(ph)
								if err == nil {
									pubkeyHash = fmt.Sprintf("%x", pubkeyHashBytes)
								} else {
									// If decoding fails, try using it as-is (might already be hex)
									pubkeyHash = ph
								}
							} else if publicKey, ok := entryMap["public_key"].(string); ok && publicKey != "" {
								// Compute pubkey_hash from public_key (decode base64 first if needed)
								var pubKeyBytes []byte
								if decoded, err := base64.StdEncoding.DecodeString(publicKey); err == nil {
									pubKeyBytes = decoded
								} else {
									pubKeyBytes = []byte(publicKey)
								}
								hash := sha256.Sum256(pubKeyBytes)
								pubkeyHash = fmt.Sprintf("%x", hash)
							}

							if pubkeyHash != "" {
								return index, pubkeyHash, true, nil // hasData = true
							}
						}
					} else {
						// Chain exists but is empty - key exists but no data
						return 0, "", false, nil // hasData = false, but no error
					}
				} else {
					// Success = false, but got response - might be key not found
					errorMsg, _ := chainResp["error"].(string)
					if errorMsg != "" {
						// Key not found - no data
						return 0, "", false, nil // hasData = false, but no error
					}
				}
			}
		} else if resp.StatusCode == http.StatusNotFound {
			// Key not found - no data
			return 0, "", false, nil // hasData = false, but no error
		} else {
			lastErr = fmt.Errorf("error from %s: status %d", endpoint, resp.StatusCode)
		}
	}

	// All endpoints failed - Raft is unavailable
	if lastErr != nil {
		return 0, "", false, lastErr
	}
	
	// No endpoints responded - Raft unavailable
	return 0, "", false, fmt.Errorf("all Raft endpoints unavailable for key: %s", keyID)
}
