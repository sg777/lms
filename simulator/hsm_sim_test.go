package simulator

import (
	"fmt"
	"testing"
)

func TestNewHSMSimulator(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	sim := NewHSMSimulator("test-hsm-1", endpoints, genesisHash)
	
	if sim == nil {
		t.Fatal("NewHSMSimulator returned nil")
	}
	
	if sim.GetID() != "test-hsm-1" {
		t.Errorf("Expected HSM ID 'test-hsm-1', got '%s'", sim.GetID())
	}
	
	stats := sim.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	
	if stats.TotalAttestations != 0 {
		t.Errorf("Expected 0 total attestations initially, got %d", stats.TotalAttestations)
	}
}

func TestHSMSimulatorStats(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	sim := NewHSMSimulator("test-hsm-1", endpoints, genesisHash)
	
	stats := sim.GetStats()
	if stats.TotalAttestations != 0 {
		t.Errorf("Expected 0 attestations, got %d", stats.TotalAttestations)
	}
	
	if stats.SuccessfulCommits != 0 {
		t.Errorf("Expected 0 successful commits, got %d", stats.SuccessfulCommits)
	}
	
	if stats.FailedCommits != 0 {
		t.Errorf("Expected 0 failed commits, got %d", stats.FailedCommits)
	}
}

func TestNewHSMSimulatorPool(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	pool := NewHSMSimulatorPool(endpoints, genesisHash, 5)
	
	if pool == nil {
		t.Fatal("NewHSMSimulatorPool returned nil")
	}
	
	if pool.GetCount() != 5 {
		t.Errorf("Expected 5 simulators, got %d", pool.GetCount())
	}
	
	sim := pool.GetSimulator(0)
	if sim == nil {
		t.Fatal("GetSimulator(0) returned nil")
	}
	
	if sim.GetID() != "hsm-sim-1" {
		t.Errorf("Expected HSM ID 'hsm-sim-1', got '%s'", sim.GetID())
	}
	
	sim = pool.GetSimulator(4)
	if sim == nil {
		t.Fatal("GetSimulator(4) returned nil")
	}
	
	if sim.GetID() != "hsm-sim-5" {
		t.Errorf("Expected HSM ID 'hsm-sim-5', got '%s'", sim.GetID())
	}
	
	// Test out of bounds
	sim = pool.GetSimulator(10)
	if sim != nil {
		t.Error("Expected nil for out of bounds index")
	}
}

func TestHSMSimulatorPoolGetAllSimulators(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	pool := NewHSMSimulatorPool(endpoints, genesisHash, 3)
	
	simulators := pool.GetAllSimulators()
	if len(simulators) != 3 {
		t.Errorf("Expected 3 simulators, got %d", len(simulators))
	}
	
	for i, sim := range simulators {
		expectedID := fmt.Sprintf("hsm-sim-%d", i+1)
		if sim.GetID() != expectedID {
			t.Errorf("Expected HSM ID '%s', got '%s'", expectedID, sim.GetID())
		}
	}
}

func TestHSMSimulatorPoolGetTotalStats(t *testing.T) {
	endpoints := []string{"http://127.0.0.1:8080"}
	genesisHash := "test-genesis-hash"
	
	pool := NewHSMSimulatorPool(endpoints, genesisHash, 3)
	
	stats := pool.GetTotalStats()
	if len(stats) != 3 {
		t.Errorf("Expected stats for 3 HSMs, got %d", len(stats))
	}
	
	for id, stat := range stats {
		if stat == nil {
			t.Errorf("Stats for %s is nil", id)
		}
		if stat.TotalAttestations != 0 {
			t.Errorf("Expected 0 attestations for %s, got %d", id, stat.TotalAttestations)
		}
	}
}

