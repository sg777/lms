package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/verifiable-state-chains/lms/models"
)

// HSMClient provides a client interface for HSM partitions to interact with the service
type HSMClient struct {
	// Service endpoints (can be any node, will auto-forward to leader)
	serviceEndpoints []string
	httpClient       *http.Client
	hsmIdentifier    string
}

// NewHSMClient creates a new HSM client
// serviceEndpoints: List of service node addresses (e.g., ["http://159.69.23.29:8080", ...])
// hsmIdentifier: Unique identifier for this HSM partition
func NewHSMClient(serviceEndpoints []string, hsmIdentifier string) *HSMClient {
	return &HSMClient{
		serviceEndpoints: serviceEndpoints,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		hsmIdentifier: hsmIdentifier,
	}
}

// GetLatestHead fetches the latest attestation head from the service
// Returns the latest committed attestation, or nil if the chain is empty
func (c *HSMClient) GetLatestHead() (*models.AttestationResponse, uint64, uint64, error) {
	var lastErr error
	
	// Try each endpoint until one succeeds
	for _, endpoint := range c.serviceEndpoints {
		url := fmt.Sprintf("%s/latest-head", endpoint)
		
		resp, err := c.httpClient.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("error from %s: status %d, body: %s", endpoint, resp.StatusCode, string(body))
			continue
		}
		
		var response models.GetLatestHeadResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			lastErr = fmt.Errorf("failed to decode response from %s: %v", endpoint, err)
			continue
		}
		
		if !response.Success {
			if response.Error != "" {
				// Chain might be empty (no attestations yet)
				if response.Error == "no attestations in chain" {
					return nil, 0, 0, nil
				}
				lastErr = fmt.Errorf("service error: %s", response.Error)
				continue
			}
		}
		
		return response.Attestation, response.RaftIndex, response.RaftTerm, nil
	}
	
	return nil, 0, 0, fmt.Errorf("all endpoints failed: %v", lastErr)
}

// ProposeAttestation submits an attestation to the service for commitment
// Returns true if committed, false if rejected, and any error
func (c *HSMClient) ProposeAttestation(attestation *models.AttestationResponse) (bool, uint64, uint64, error) {
	// Prepare request
	req := models.ProposeAttestationRequest{
		Attestation:  attestation,
		HSMIdentifier: c.hsmIdentifier,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return false, 0, 0, fmt.Errorf("failed to marshal request: %v", err)
	}
	
	var lastErr error
	
	// Try each endpoint until one succeeds
	for _, endpoint := range c.serviceEndpoints {
		url := fmt.Sprintf("%s/propose", endpoint)
		
		resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		body, _ := io.ReadAll(resp.Body)
		
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("error from %s: status %d, body: %s", endpoint, resp.StatusCode, string(body))
			continue
		}
		
		var response models.ProposeAttestationResponse
		if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
			lastErr = fmt.Errorf("failed to decode response from %s: %v", endpoint, err)
			continue
		}
		
		if !response.Success {
			// Attestation was rejected
			return false, 0, 0, fmt.Errorf("attestation rejected: %s", response.Error)
		}
		
		if !response.Committed {
			// Attestation was accepted but not committed (shouldn't happen in normal operation)
			return false, 0, 0, fmt.Errorf("attestation accepted but not committed")
		}
		
		// Success!
		return true, response.RaftIndex, response.RaftTerm, nil
	}
	
	return false, 0, 0, fmt.Errorf("all endpoints failed: %v", lastErr)
}

// HealthCheck checks if the service is healthy
func (c *HSMClient) HealthCheck() (*models.HealthCheckResponse, error) {
	var lastErr error
	
	for _, endpoint := range c.serviceEndpoints {
		url := fmt.Sprintf("%s/health", endpoint)
		
		resp, err := c.httpClient.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("error from %s: status %d, body: %s", endpoint, resp.StatusCode, string(body))
			continue
		}
		
		var response models.HealthCheckResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			lastErr = fmt.Errorf("failed to decode response from %s: %v", endpoint, err)
			continue
		}
		
		return &response, nil
	}
	
	return nil, fmt.Errorf("all endpoints failed: %v", lastErr)
}

// GetLeaderInfo gets information about the current leader
func (c *HSMClient) GetLeaderInfo() (*models.LeaderInfoResponse, error) {
	var lastErr error
	
	for _, endpoint := range c.serviceEndpoints {
		url := fmt.Sprintf("%s/leader", endpoint)
		
		resp, err := c.httpClient.Get(url)
		if err != nil {
			lastErr = fmt.Errorf("failed to connect to %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("error from %s: status %d, body: %s", endpoint, resp.StatusCode, string(body))
			continue
		}
		
		var response models.LeaderInfoResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			lastErr = fmt.Errorf("failed to decode response from %s: %v", endpoint, err)
			continue
		}
		
		return &response, nil
	}
	
	return nil, fmt.Errorf("all endpoints failed: %v", lastErr)
}

