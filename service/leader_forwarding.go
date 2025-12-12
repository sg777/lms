package service

import (
	"fmt"
	"net/http"
	"io"
	"bytes"
	
	"github.com/hashicorp/raft"
)

// LeaderForwarder handles leader detection and request forwarding
type LeaderForwarder struct {
	raft     *raft.Raft
	config   *Config
	client   *http.Client
}

// NewLeaderForwarder creates a new leader forwarder
func NewLeaderForwarder(r *raft.Raft, cfg *Config) *LeaderForwarder {
	return &LeaderForwarder{
		raft:   r,
		config: cfg,
		client: &http.Client{
			Timeout: cfg.RequestTimeout,
		},
	}
}

// IsLeader checks if the current node is the leader
func (lf *LeaderForwarder) IsLeader() bool {
	return lf.raft.State() == raft.Leader
}

// GetLeaderID returns the current leader's ID
func (lf *LeaderForwarder) GetLeaderID() string {
	leaderAddr := lf.raft.Leader()
	if leaderAddr == "" {
		return ""
	}
	
	// Find which node has this address
	for _, node := range lf.config.ClusterNodes {
		if string(leaderAddr) == node.Address {
			return node.ID
		}
	}
	return ""
}

// GetLeaderAPIAddress returns the HTTP API address of the current leader
func (lf *LeaderForwarder) GetLeaderAPIAddress() string {
	leaderID := lf.GetLeaderID()
	if leaderID == "" {
		return ""
	}
	
	node := lf.config.GetNodeByID(leaderID)
	if node == nil {
		return ""
	}
	
	// Extract IP from Raft address and construct API address
	ip := node.Address[:len(node.Address)-5] // Remove ":7000"
	return fmt.Sprintf("http://%s:%d", ip, node.APIPort)
}

// ForwardRequest forwards an HTTP request to the leader
func (lf *LeaderForwarder) ForwardRequest(w http.ResponseWriter, r *http.Request, path string) error {
	leaderAddr := lf.GetLeaderAPIAddress()
	if leaderAddr == "" {
		http.Error(w, "No leader available", http.StatusServiceUnavailable)
		return fmt.Errorf("no leader available")
	}
	
	// Construct the full URL
	url := leaderAddr + path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	
	// Create a new request
	var body io.Reader
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return err
		}
		body = bytes.NewReader(bodyBytes)
	}
	
	req, err := http.NewRequest(r.Method, url, body)
	if err != nil {
		http.Error(w, "Failed to create forward request", http.StatusInternalServerError)
		return err
	}
	
	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	
	// Forward the request
	resp, err := lf.client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to forward request: %v", err), http.StatusBadGateway)
		return err
	}
	defer resp.Body.Close()
	
	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	
	// Set status code
	w.WriteHeader(resp.StatusCode)
	
	// Copy response body
	_, err = io.Copy(w, resp.Body)
	return err
}

// RedirectToLeader sends a redirect response with leader information
func (lf *LeaderForwarder) RedirectToLeader(w http.ResponseWriter, r *http.Request) {
	leaderAddr := lf.GetLeaderAPIAddress()
	if leaderAddr == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"leader_id":"","is_leader":false,"error":"No leader available"}`))
		return
	}
	
	// Return leader info in response
	leaderID := lf.GetLeaderID()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTemporaryRedirect)
	w.Write([]byte(fmt.Sprintf(`{"leader_id":"%s","leader_addr":"%s","is_leader":false}`, leaderID, leaderAddr)))
}

