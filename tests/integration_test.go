package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/verifiable-state-chains/lms/client"
	"github.com/verifiable-state-chains/lms/models"
	"github.com/verifiable-state-chains/lms/simulator"
)

// TestService represents a running service instance
type TestService struct {
	NodeID   string
	Address  string
	APIPort  int
	RaftDir  string
	Process  *exec.Cmd
	Context  context.Context
	Cancel   context.CancelFunc
}

// TestCluster represents a test Raft cluster
type TestCluster struct {
	Services []*TestService
	GenesisHash string
}

// StartTestCluster starts a test cluster with the specified number of nodes
// NOTE: These tests require the service binary to be built and may have port conflicts.
// They are marked to skip in short mode. For reliable testing, use manual cluster setup.
func StartTestCluster(nodeCount int, genesisHash string) (*TestCluster, error) {
	if nodeCount < 1 || nodeCount > 3 {
		return nil, fmt.Errorf("nodeCount must be between 1 and 3")
	}

	cluster := &TestCluster{
		Services:    make([]*TestService, 0, nodeCount),
		GenesisHash: genesisHash,
	}

	// Create temporary directories for Raft data
	baseDir, err := os.MkdirTemp("", "lms-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}

	// Build cluster node addresses for bootstrap
	clusterNodes := make([]string, nodeCount)
	for i := 0; i < nodeCount; i++ {
		raftPort := 7000 + i
		clusterNodes[i] = fmt.Sprintf("test-node-%d,127.0.0.1:%d", i+1, raftPort)
	}

	// Start nodes
	for i := 0; i < nodeCount; i++ {
		nodeID := fmt.Sprintf("test-node-%d", i+1)
		raftPort := 7000 + i
		apiPort := 8080 + i
		raftDir := filepath.Join(baseDir, nodeID)

		ctx, cancel := context.WithCancel(context.Background())

		// Build service binary path (assuming it's in the project root)
		binaryPath := "./lms-service"
		if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
			// Try to find it
			binaryPath = "../lms-service"
		}

		cmd := exec.CommandContext(ctx, binaryPath,
			"-id", nodeID,
			"-addr", fmt.Sprintf("127.0.0.1:%d", raftPort),
			"-api-port", fmt.Sprintf("%d", apiPort),
			"-raft-dir", raftDir,
			"-genesis-hash", genesisHash,
		)

		if i == 0 {
			// First node bootstraps
			cmd.Args = append(cmd.Args, "-bootstrap")
		}

		// Suppress output for cleaner test output
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Start(); err != nil {
			// Cleanup
			cancel()
			for _, svc := range cluster.Services {
				svc.Cancel()
			}
			return nil, fmt.Errorf("failed to start node %s: %v", nodeID, err)
		}

		service := &TestService{
			NodeID:  nodeID,
			Address: fmt.Sprintf("127.0.0.1:%d", raftPort),
			APIPort: apiPort,
			RaftDir: raftDir,
			Process: cmd,
			Context: ctx,
			Cancel:  cancel,
		}

		cluster.Services = append(cluster.Services, service)

		// Wait a bit for node to start
		time.Sleep(1 * time.Second)
	}

	// Wait for cluster to stabilize
	time.Sleep(3 * time.Second)

	return cluster, nil
}

// Stop stops all services in the cluster
func (c *TestCluster) Stop() error {
	for _, svc := range c.Services {
		if svc.Cancel != nil {
			svc.Cancel()
		}
		if svc.Process != nil {
			svc.Process.Wait()
		}
	}
	return nil
}

// GetServiceEndpoints returns API endpoints for all services
func (c *TestCluster) GetServiceEndpoints() []string {
	endpoints := make([]string, 0, len(c.Services))
	for _, svc := range c.Services {
		endpoints = append(endpoints, fmt.Sprintf("http://127.0.0.1:%d", svc.APIPort))
	}
	return endpoints
}

// TestSingleHSMWorkflow tests a single HSM committing attestations
// NOTE: This test requires manual setup or a properly configured test environment.
// It may fail due to port conflicts or bootstrap issues.
func TestSingleHSMWorkflow(t *testing.T) {
	// Skip by default - these tests require manual cluster setup
	t.Skip("Skipping - requires manual cluster setup. See TESTING_GUIDE.md for manual testing instructions")
	
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	genesisHash := "test-genesis-hash-integration"
	cluster, err := StartTestCluster(3, genesisHash)
	if err != nil {
		t.Fatalf("Failed to start test cluster: %v", err)
	}
	defer cluster.Stop()

	endpoints := cluster.GetServiceEndpoints()
	hsmClient := client.NewHSMClient(endpoints, "test-hsm-1")
	protocol := client.NewHSMProtocol(hsmClient, genesisHash)

	// Sync state
	if err := protocol.SyncState(); err != nil {
		t.Fatalf("Failed to sync state: %v", err)
	}

	// Generate and commit attestations
	for i := 0; i < 5; i++ {
		messageHash := fmt.Sprintf("test-message-%d", i)
		committed, _, _, err := protocol.CompleteWorkflow(
			messageHash,
			"LMS_ATTEST_POLICY",
			"PS256",
			"mock-signature",
			"mock-certificate",
			5*time.Second,
		)

		if err != nil || !committed {
			t.Fatalf("Failed to commit attestation %d: %v", i, err)
		}

		time.Sleep(200 * time.Millisecond)
	}

	// Verify state
	state := protocol.GetState()
	if state.CurrentLMSIndex != 4 {
		t.Errorf("Expected LMS index 4, got %d", state.CurrentLMSIndex)
	}
}

// TestMultipleHSMsConcurrent tests multiple HSMs committing attestations concurrently
// NOTE: This test requires manual setup or a properly configured test environment.
func TestMultipleHSMsConcurrent(t *testing.T) {
	// Skip by default - these tests require manual cluster setup
	t.Skip("Skipping - requires manual cluster setup. See TESTING_GUIDE.md for manual testing instructions")
	
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	genesisHash := "test-genesis-hash-concurrent"
	cluster, err := StartTestCluster(3, genesisHash)
	if err != nil {
		t.Fatalf("Failed to start test cluster: %v", err)
	}
	defer cluster.Stop()

	endpoints := cluster.GetServiceEndpoints()

	// Create HSM simulator pool
	pool := simulator.NewHSMSimulatorPool(endpoints, genesisHash, 3)

	// Run concurrent attestations
	err = pool.RunConcurrentAttestations(5, "concurrent-test")
	if err != nil {
		t.Fatalf("Concurrent attestations failed: %v", err)
	}

	// Check stats
	stats := pool.GetTotalStats()
	totalSuccess := 0
	for _, stat := range stats {
		totalSuccess += stat.SuccessfulCommits
	}

	expected := 3 * 5 // 3 HSMs * 5 attestations each
	if totalSuccess < expected {
		t.Errorf("Expected %d successful commits, got %d", expected, totalSuccess)
	}
}


// TestValidationIntegration tests that validation works in integration
// NOTE: This test requires manual setup or a properly configured test environment.
func TestValidationIntegration(t *testing.T) {
	// Skip by default - these tests require manual cluster setup
	t.Skip("Skipping - requires manual cluster setup. See TESTING_GUIDE.md for manual testing instructions")
	
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	genesisHash := "test-genesis-hash-validation"
	cluster, err := StartTestCluster(3, genesisHash)
	if err != nil {
		t.Fatalf("Failed to start test cluster: %v", err)
	}
	defer cluster.Stop()

	endpoints := cluster.GetServiceEndpoints()
	hsmClient := client.NewHSMClient(endpoints, "test-hsm-1")
	protocol := client.NewHSMProtocol(hsmClient, genesisHash)

	// Sync state
	if err := protocol.SyncState(); err != nil {
		t.Fatalf("Failed to sync state: %v", err)
	}

	// Try to commit an attestation with invalid previous_hash
	// This should be rejected by validation
	payload := &models.ChainedPayload{
		PreviousHash:   "invalid-hash",
		LMSIndex:       1,
		MessageSigned:  "test-message",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	attestation := &models.AttestationResponse{}
	attestation.SetChainedPayload(payload)
	attestation.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	attestation.AttestationResponse.Data.Encoding = "base64"
	attestation.AttestationResponse.Signature.Value = "mock-signature"
	attestation.AttestationResponse.Signature.Encoding = "base64"
	attestation.AttestationResponse.Certificate.Value = "mock-certificate"
	attestation.AttestationResponse.Certificate.Encoding = "pem"

	committed, _, _, err := protocol.CommitAttestation(attestation, 5*time.Second)
	if committed {
		t.Error("Expected invalid attestation to be rejected, but it was committed")
	}
	if err == nil {
		t.Error("Expected error when committing invalid attestation")
	}
}

