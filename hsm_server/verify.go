package hsm_server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

// VerifyRequest represents the verification request
type VerifyRequest struct {
	KeyID     string                 `json:"key_id,omitempty"` // Optional: if provided, use this key's pubkey
	Signature *StructuredSignature   `json:"signature"`        // Structured signature with pubkey, index, signature
	Message   string                 `json:"message"`          // Original message to verify
	UserID    string                 `json:"user_id,omitempty"` // User ID from JWT (added by explorer proxy)
}


// VerifyResponse represents the verification response
type VerifyResponse struct {
	Success bool   `json:"success"`
	Valid   bool   `json:"valid,omitempty"`   // Whether signature is valid
	Error   string `json:"error,omitempty"`
}

// handleVerify handles signature verification requests
func (s *HSMServer) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := VerifyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Validate required fields
	if req.Signature == nil {
		response := VerifyResponse{
			Success: false,
			Error:   "signature is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	if req.Message == "" {
		response := VerifyResponse{
			Success: false,
			Error:   "message is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get public key - either from signature or from key_id
	var publicKeyBytes []byte
	if req.KeyID != "" {
		// Use key_id's public key
		s.mu.RLock()
		key, exists := s.keys[req.KeyID]
		s.mu.RUnlock()

		if !exists {
			// Try database
			dbKey, err := s.db.GetKey(req.KeyID)
			if err != nil || dbKey == nil {
				response := VerifyResponse{
					Success: false,
					Error:   fmt.Sprintf("Key %s not found", req.KeyID),
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(response)
				return
			}
			key = dbKey
		}

		// Verify ownership if user_id provided
		if req.UserID != "" {
			if key.UserID != "" && key.UserID != req.UserID {
				response := VerifyResponse{
					Success: false,
					Error:   "You do not have permission to use this key",
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		publicKeyBytes = key.PublicKey
	} else {
		// Use public key from signature
		var err error
		publicKeyBytes, err = base64.StdEncoding.DecodeString(req.Signature.PublicKey)
		if err != nil {
			response := VerifyResponse{
				Success: false,
				Error:   fmt.Sprintf("Invalid public key encoding: %v", err),
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Decode signature
	signatureBytes, err := base64.StdEncoding.DecodeString(req.Signature.Signature)
	if err != nil {
		response := VerifyResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid signature encoding: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Verify signature
	messageBytes := []byte(req.Message)
	valid, err := lms_wrapper.VerifySignature(publicKeyBytes, messageBytes, signatureBytes)
	if err != nil {
		response := VerifyResponse{
			Success: false,
			Error:   fmt.Sprintf("Verification failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := VerifyResponse{
		Success: true,
		Valid:   valid,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

