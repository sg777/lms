package explorer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// handleWalletTotalBalance returns the total balance across all user's wallets
func (s *ExplorerServer) handleWalletTotalBalance(w http.ResponseWriter, r *http.Request) {
	log.Printf("[TOTAL_BALANCE] ===== HANDLER CALLED ===== URL: %s, Method: %s, RemoteAddr: %s", r.URL.String(), r.Method, r.RemoteAddr)

	// CRITICAL: Always set JSON header first (before any writes)
	w.Header().Set("Content-Type", "application/json")

	// Safety: prevent panics from surfacing as 500 HTML
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[TOTAL_BALANCE] PANIC RECOVERED: %v", rec)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":       false,
				"error":         fmt.Sprintf("internal error: %v", rec),
				"total_balance": 0.0,
			})
		}
	}()

	if r.Method != http.MethodGet {
		log.Printf("[TOTAL_BALANCE] Method not allowed: %s (expected GET)", r.Method)
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
		log.Printf("[TOTAL_BALANCE] ERROR: No token found")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}

	claims, err := ValidateToken(tokenString)
	if err != nil {
		log.Printf("[TOTAL_BALANCE] ERROR: Token validation failed: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Unauthorized",
		})
		return
	}
	userID := claims.UserID

	// Get user's wallets
	log.Printf("[TOTAL_BALANCE] Getting wallets for user: %s", userID)
	wallets, err := s.walletDB.GetWalletsByUserID(userID)
	if err != nil {
		log.Printf("[TOTAL_BALANCE] ERROR: Failed to get wallets: %v", err)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       false,
			"error":         fmt.Sprintf("Failed to get wallets: %v", err),
			"total_balance": 0.0,
		})
		return
	}

	if len(wallets) == 0 {
		log.Printf("[TOTAL_BALANCE] No wallets found for user: %s", userID)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       true,
			"total_balance": 0.0,
			"message":       "No wallets found",
		})
		return
	}

	// Get balance for each wallet
	log.Printf("[TOTAL_BALANCE] Fetching balances for %d wallets", len(wallets))
	verusClient := newVerusClientFromEnv()

	totalBalance := 0.0
	walletBalances := make([]map[string]interface{}, 0)
	var balanceErrors []string

	for _, wallet := range wallets {
		balance, err := verusClient.GetBalance(wallet.Address)
		if err != nil {
			// Record error but continue; do not fail the whole request
			log.Printf("[TOTAL_BALANCE] Error getting balance for %s: %v", wallet.Address, err)
			balanceErrors = append(balanceErrors, fmt.Sprintf("%s: %v", wallet.Address, err))
			balance = 0.0
		}
		totalBalance += balance
		walletBalances = append(walletBalances, map[string]interface{}{
			"address": wallet.Address,
			"balance": balance,
		})
	}

	log.Printf("[TOTAL_BALANCE] Success - Total balance: %f CHIPS, Wallets: %d, Errors: %d", totalBalance, len(wallets), len(balanceErrors))
	response := map[string]interface{}{
		"success":         true,
		"total_balance":   totalBalance,
		"wallet_balances": walletBalances,
	}
	if len(balanceErrors) > 0 {
		response["errors"] = balanceErrors
	}

	json.NewEncoder(w).Encode(response)
}
