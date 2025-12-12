package hsm_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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

