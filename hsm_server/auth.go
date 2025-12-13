package hsm_server

import (
	"fmt"
	"net/http"

	"github.com/verifiable-state-chains/lms/explorer"
)

// extractTokenFromHeader extracts JWT token from Authorization header
func extractTokenFromHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	// Format: "Bearer <token>"
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}

	return ""
}

// getUserIdFromRequest extracts user ID from JWT token in request
func getUserIdFromRequest(r *http.Request) (string, error) {
	tokenString := extractTokenFromHeader(r)
	if tokenString == "" {
		return "", fmt.Errorf("no authorization token")
	}

	claims, err := explorer.ValidateToken(tokenString)
	if err != nil {
		return "", fmt.Errorf("invalid token: %v", err)
	}

	return claims.UserID, nil
}

