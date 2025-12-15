package explorer

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// handleWalletTotalBalance returns the total balance across all user's wallets
func (s *ExplorerServer) handleWalletTotalBalance(w http.ResponseWriter, r *http.Request) {
	// Safety: prevent panics from surfacing as 500 HTML
	defer func() {
		if rec := recover(); rec != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success":       false,
				"error":         fmt.Sprintf("internal error: %v", rec),
				"total_balance": 0.0,
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

	// Get user's wallets
	wallets, err := s.walletDB.GetWalletsByUserID(userID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       false,
			"error":         fmt.Sprintf("Failed to get wallets: %v", err),
			"total_balance": 0.0,
		})
		return
	}

	if len(wallets) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":       true,
			"total_balance": 0.0,
			"message":       "No wallets found",
		})
		return
	}

	// Get balance for each wallet
	verusClient := newVerusClientFromEnv()

	totalBalance := 0.0
	walletBalances := make([]map[string]interface{}, 0)
	var balanceErrors []string

	for _, wallet := range wallets {
		balance, err := verusClient.GetBalance(wallet.Address)
		if err != nil {
			// Record error but continue; do not fail the whole request
			balanceErrors = append(balanceErrors, fmt.Sprintf("%s: %v", wallet.Address, err))
			balance = 0.0
		}
		totalBalance += balance
		walletBalances = append(walletBalances, map[string]interface{}{
			"address": wallet.Address,
			"balance": balance,
		})
	}

	w.Header().Set("Content-Type", "application/json")
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
