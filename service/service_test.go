package service

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.NodeID != "node1" {
		t.Errorf("Expected NodeID 'node1', got '%s'", cfg.NodeID)
	}
	
	if cfg.APIPort != 8080 {
		t.Errorf("Expected APIPort 8080, got %d", cfg.APIPort)
	}
	
	if len(cfg.ClusterNodes) != 3 {
		t.Errorf("Expected 3 cluster nodes, got %d", len(cfg.ClusterNodes))
	}
}

func TestGetNodeByID(t *testing.T) {
	cfg := DefaultConfig()
	
	node := cfg.GetNodeByID("node1")
	if node == nil {
		t.Fatal("Expected to find node1")
	}
	
	if node.ID != "node1" {
		t.Errorf("Expected node ID 'node1', got '%s'", node.ID)
	}
	
	node = cfg.GetNodeByID("nonexistent")
	if node != nil {
		t.Error("Expected nil for nonexistent node")
	}
}

func TestGetAPIAddress(t *testing.T) {
	cfg := DefaultConfig()
	
	addr := cfg.GetAPIAddress("node1")
	expected := "159.69.23.29:8080"
	if addr != expected {
		t.Errorf("Expected API address '%s', got '%s'", expected, addr)
	}
}

