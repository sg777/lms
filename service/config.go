package service

import (
	"fmt"
	"time"
)

// Config holds the service configuration
type Config struct {
	// Node configuration
	NodeID      string
	NodeAddr    string // IP:port for Raft transport (e.g., "159.69.23.29:7000")
	APIPort     int    // HTTP API port (e.g., 8080)
	RaftPort    int    // Raft transport port (e.g., 7000)
	
	// Raft configuration
	RaftDir     string // Directory for Raft data
	Bootstrap   bool   // Whether to bootstrap the cluster
	
	// Cluster configuration
	ClusterNodes []ClusterNode // All nodes in the cluster
	
	// Timeouts
	RequestTimeout time.Duration // Timeout for Raft operations
	LeaderTimeout  time.Duration // Timeout for leader election
}

// ClusterNode represents a node in the Raft cluster
type ClusterNode struct {
	ID      string // Node ID (e.g., "node1")
	Address string // Raft address (e.g., "159.69.23.29:7000")
	APIPort int    // API port (e.g., 8080)
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		NodeID:        "node1",
		NodeAddr:      "159.69.23.29:7000",
		APIPort:       8080,
		RaftPort:      7000,
		RaftDir:       "./raft-data",
		Bootstrap:     false,
		RequestTimeout: 5 * time.Second,
		LeaderTimeout:  10 * time.Second,
		ClusterNodes: []ClusterNode{
			{ID: "node1", Address: "159.69.23.29:7000", APIPort: 8080},
			{ID: "node2", Address: "159.69.23.30:7000", APIPort: 8080},
			{ID: "node3", Address: "159.69.23.31:7000", APIPort: 8080},
		},
	}
}

// GetNodeByID returns the cluster node with the given ID
func (c *Config) GetNodeByID(id string) *ClusterNode {
	for _, node := range c.ClusterNodes {
		if node.ID == id {
			return &node
		}
	}
	return nil
}

// GetAPIAddress returns the full API address for a node
func (c *Config) GetAPIAddress(nodeID string) string {
	node := c.GetNodeByID(nodeID)
	if node == nil {
		return ""
	}
	// Extract IP from Raft address
	ip := node.Address[:len(node.Address)-5] // Remove ":7000"
	return fmt.Sprintf("%s:%d", ip, node.APIPort)
}

