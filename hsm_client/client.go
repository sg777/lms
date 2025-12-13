package hsm_client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/verifiable-state-chains/lms/lms_wrapper"
)

// HSMClient is a client for interacting with HSM server
type HSMClient struct {
	serverURL string
	httpClient *http.Client
}

// LMSKey represents an LMS key
type LMSKey struct {
	KeyID   string `json:"key_id"`
	Index   uint64 `json:"index"`
	Created string `json:"created"`
}

// NewHSMClient creates a new HSM client
func NewHSMClient(serverURL string) *HSMClient {
	return &HSMClient{
		serverURL: serverURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GenerateKeyResponse is the response from generating a key
type GenerateKeyResponse struct {
	Success bool   `json:"success"`
	KeyID   string `json:"key_id"`
	Index   uint64 `json:"index"`
	Error   string `json:"error,omitempty"`
}

// ListKeysResponse is the response for listing keys
type ListKeysResponse struct {
	Success bool     `json:"success"`
	Keys    []LMSKey `json:"keys"`
	Count   int      `json:"count"`
	Error   string   `json:"error,omitempty"`
}

// GenerateKeyRequest is the request to generate a key
type GenerateKeyRequest struct {
	KeyID string `json:"key_id,omitempty"`
}

// GenerateKey generates a new LMS key on the HSM server
// If keyID is empty, server will generate one
func (c *HSMClient) GenerateKey(keyID string) (*LMSKey, error) {
	req := GenerateKeyRequest{
		KeyID: keyID,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	url := fmt.Sprintf("%s/generate_key", c.serverURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HSM server: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		var errorResp GenerateKeyResponse
		json.Unmarshal(body, &errorResp)
		return nil, fmt.Errorf("HSM server error: %s", errorResp.Error)
	}
	
	var response GenerateKeyResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	
	if !response.Success {
		return nil, fmt.Errorf("key generation failed: %s", response.Error)
	}
	
	return &LMSKey{
		KeyID:   response.KeyID,
		Index:   response.Index,
		Created: "", // Server doesn't return this in response
	}, nil
}

// ListKeys lists all available keys on the HSM server
func (c *HSMClient) ListKeys() ([]LMSKey, error) {
	url := fmt.Sprintf("%s/list_keys", c.serverURL)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HSM server: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		var errorResp ListKeysResponse
		json.Unmarshal(body, &errorResp)
		return nil, fmt.Errorf("HSM server error: %s", errorResp.Error)
	}
	
	var response ListKeysResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	
	if !response.Success {
		return nil, fmt.Errorf("list keys failed: %s", response.Error)
	}
	
	return response.Keys, nil
}

// SignRequest is the request to sign a message
type SignRequest struct {
	KeyID   string `json:"key_id"`
	Message string `json:"message"`
}

// SignResponse is the response from signing
type SignResponse struct {
	Success   bool   `json:"success"`
	KeyID     string `json:"key_id,omitempty"`
	Index     uint64 `json:"index,omitempty"`
	Signature string `json:"signature"`
	Error     string `json:"error,omitempty"`
}

// Sign signs a message with the specified key_id
func (c *HSMClient) Sign(keyID, message string) (*SignResponse, error) {
	req := SignRequest{
		KeyID:   keyID,
		Message: message,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	url := fmt.Sprintf("%s/sign", c.serverURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to HSM server: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		var errorResp SignResponse
		json.Unmarshal(body, &errorResp)
		return nil, fmt.Errorf("HSM server error: %s", errorResp.Error)
	}
	
	var response SignResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}
	
	if !response.Success {
		return nil, fmt.Errorf("sign failed: %s", response.Error)
	}
	
	return &response, nil
}

// QueryKeyIndex queries the Raft cluster for a key_id's last index
func QueryKeyIndex(raftEndpoint, keyID string) (uint64, bool, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	url := fmt.Sprintf("%s/key/%s/index", raftEndpoint, keyID)
	resp, err := client.Get(url)
	if err != nil {
		return 0, false, fmt.Errorf("failed to connect to Raft: %v", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		return 0, false, fmt.Errorf("Raft error: status %d, body: %s", resp.StatusCode, string(body))
	}
	
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, false, fmt.Errorf("failed to decode response: %v", err)
	}
	
	success, _ := response["success"].(bool)
	if !success {
		errorMsg, _ := response["error"].(string)
		return 0, false, fmt.Errorf("query failed: %s", errorMsg)
	}
	
	exists, _ := response["exists"].(bool)
	if !exists {
		return 0, false, nil
	}
	
	index, ok := response["index"].(float64)
	if !ok {
		return 0, false, fmt.Errorf("invalid index in response")
	}
	
	return uint64(index), true, nil
}

// DeleteAllKeysResponse is the response from deleting all keys
type DeleteAllKeysResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// DeleteAllKeys deletes all keys from the HSM server
func (c *HSMClient) DeleteAllKeys() error {
	url := fmt.Sprintf("%s/delete_all_keys", c.serverURL)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to HSM server: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errorResp DeleteAllKeysResponse
		json.Unmarshal(body, &errorResp)
		return fmt.Errorf("HSM server error: %s", errorResp.Error)
	}

	var response DeleteAllKeysResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}

	if !response.Success {
		return fmt.Errorf("delete all keys failed: %s", response.Error)
	}

	return nil
}

// KeyChainEntry represents a single entry in the hash chain
type KeyChainEntry struct {
	KeyID       string `json:"key_id"`
	Index       uint64 `json:"index"`
	PreviousHash string `json:"previous_hash"`
	Hash        string `json:"hash"`
	Signature   string `json:"signature"`
	PublicKey   string `json:"public_key"`
	RaftIndex   uint64 `json:"raft_index,omitempty"`
	RaftTerm    uint64 `json:"raft_term,omitempty"`
}

// ChainVerification represents the result of chain integrity verification
type ChainVerification struct {
	Valid      bool   `json:"valid"`
	Error      string `json:"error,omitempty"`
	BreakIndex int    `json:"break_index,omitempty"` // Index of entry where chain breaks (-1 if no break)
}

// KeyChainResponse is the response from getting the full chain
type KeyChainResponse struct {
	Success      bool              `json:"success"`
	KeyID        string            `json:"key_id"`
	Exists       bool              `json:"exists"`
	Chain        []KeyChainEntry   `json:"chain"`
	Count        int               `json:"count"`
	Verification *ChainVerification `json:"verification,omitempty"`
	Message      string            `json:"message,omitempty"`
	Error        string            `json:"error,omitempty"`
}

// GetKeyChain retrieves the full hash chain for a key_id from the Raft cluster
func GetKeyChain(raftEndpoint, keyID string) (*KeyChainResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	
	url := fmt.Sprintf("%s/key/%s/chain", raftEndpoint, keyID)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Raft: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var errorResp KeyChainResponse
		json.Unmarshal(body, &errorResp)
		return nil, fmt.Errorf("Raft error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var response KeyChainResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("query failed: %s", response.Error)
	}

	return &response, nil
}

// VerifySignature verifies an LMS signature
// Returns true if signature is valid, false otherwise
func VerifySignature(publicKey []byte, message string, signatureBase64 string) (bool, error) {
	// Decode signature from base64
	signature, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %v", err)
	}

	// Use LMS wrapper to verify
	return lms_wrapper.VerifySignature(publicKey, []byte(message), signature)
}

