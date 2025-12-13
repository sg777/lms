package explorer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
		http.Error(w, fmt.Sprintf("Failed to connect to HSM server: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("Failed to connect to HSM server: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("Failed to connect to HSM server: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("Failed to connect to HSM server: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("Failed to connect to HSM server: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("Failed to connect to HSM server: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
