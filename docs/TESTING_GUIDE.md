# Complete Testing Guide

This guide covers all ways to test the Verifiable State Chains system.

## Table of Contents

1. [Unit Tests](#unit-tests)
2. [Integration Tests](#integration-tests)
3. [Manual Testing on 3-Node Cluster](#manual-testing-on-3-node-cluster)
4. [HSM Simulator Testing](#hsm-simulator-testing)
5. [Testing Specific Scenarios](#testing-specific-scenarios)

---

## Unit Tests

### Run All Unit Tests

```bash
cd /root/lms
go test ./... -v
```

### Test Individual Modules

```bash
# Test data models
go test ./models -v

# Test FSM
go test ./fsm -v

# Test client library
go test ./client -v

# Test validation layer
go test ./validation -v

# Test simulator
go test ./simulator -v
```

### Expected Results

All unit tests should pass:
- ✅ Models: Serialization, deserialization, hashing
- ✅ FSM: Hash chain integrity, monotonicity
- ✅ Client: Protocol workflow, state management
- ✅ Validation: All validation rules
- ✅ Simulator: Simulator creation and management

---

## Integration Tests

### Simple Integration Tests (No Service Required)

These tests don't require a running service:

```bash
cd /root/lms
go test ./tests -v -run "TestHSMClientBasic|TestHSMProtocolBasic|TestHSMProtocolCreate|TestHSMSimulator"
```

### Full Integration Tests (Requires Running Service)

**Note**: These tests require a running service cluster. They are documented but may need manual setup.

```bash
# These tests will attempt to start services automatically
# May require additional setup for port management
go test ./tests -v -run "TestSingleHSMWorkflow|TestMultipleHSMsConcurrent"
```

---

## Manual Testing on 3-Node Cluster

### Prerequisites

- 3 nodes with IPs: 159.69.23.29, 159.69.23.30, 159.69.23.31
- All nodes accessible on ports 7000 (Raft) and 8080 (API)
- `lms-service` binary built and available on all nodes

### Step 1: Build the Service

```bash
cd /root/lms
go build -o lms-service .
```

Copy `lms-service` to all 3 nodes.

### Step 2: Clear Previous Data (If Needed)

```bash
# On each node
./clear_raft_data.sh
# Or manually:
rm -rf raft-data-*
```

### Step 3: Start the Cluster

**Node 3 (Bootstrap - Start First):**
```bash
./lms-service -id node3 -addr 159.69.23.31:7000 -api-port 8080 -bootstrap
```

**Node 1 (Start Second):**
```bash
./lms-service -id node1 -addr 159.69.23.29:7000 -api-port 8080
```

**Node 2 (Start Third):**
```bash
./lms-service -id node2 -addr 159.69.23.30:7000 -api-port 8080
```

### Step 4: Verify Cluster Health

```bash
# Check health on each node
curl http://159.69.23.29:8080/health
curl http://159.69.23.30:8080/health
curl http://159.69.23.31:8080/health

# Check leader
curl http://159.69.23.29:8080/leader
```

### Step 5: Test Basic Operations

**Send a message (via CLI):**
- On any node, type a message in the CLI
- Example: `test message 1`

**List all messages:**
- Type `list` in the CLI
- Or: `curl http://159.69.23.29:8080/list`

**Check latest head:**
```bash
curl http://159.69.23.29:8080/latest-head
```

---

## HSM Simulator Testing

### Basic HSM Simulator Usage

Create a simple test program:

```go
package main

import (
    "fmt"
    "log"
    "github.com/verifiable-state-chains/lms/simulator"
)

func main() {
    endpoints := []string{
        "http://159.69.23.29:8080",
        "http://159.69.23.30:8080",
        "http://159.69.23.31:8080",
    }
    
    genesisHash := "lms_genesis_hash_verifiable_state_chains"
    
    // Create a single HSM simulator
    sim := simulator.NewHSMSimulator("hsm-1", endpoints, genesisHash)
    
    // Sync state
    if err := sim.SyncState(); err != nil {
        log.Fatalf("Failed to sync state: %v", err)
    }
    
    // Generate attestations
    for i := 0; i < 5; i++ {
        messageHash := fmt.Sprintf("test-message-%d", i)
        success, err := sim.GenerateAttestation(messageHash)
        if !success {
            log.Printf("Failed to generate attestation %d: %v", i, err)
        } else {
            log.Printf("Successfully generated attestation %d", i)
        }
    }
    
    // Print statistics
    stats := sim.GetStats()
    fmt.Printf("Total: %d, Success: %d, Failed: %d\n",
        stats.TotalAttestations,
        stats.SuccessfulCommits,
        stats.FailedCommits)
}
```

### Multiple HSMs Concurrent

```go
package main

import (
    "fmt"
    "log"
    "github.com/verifiable-state-chains/lms/simulator"
)

func main() {
    endpoints := []string{
        "http://159.69.23.29:8080",
        "http://159.69.23.30:8080",
        "http://159.69.23.31:8080",
    }
    
    genesisHash := "lms_genesis_hash_verifiable_state_chains"
    
    // Create pool of 3 HSMs
    pool := simulator.NewHSMSimulatorPool(endpoints, genesisHash, 3)
    
    // Run 10 concurrent attestations per HSM
    err := pool.RunConcurrentAttestations(10, "concurrent-test")
    if err != nil {
        log.Printf("Errors occurred: %v", err)
    }
    
    // Print statistics for each HSM
    stats := pool.GetTotalStats()
    for hsmID, stat := range stats {
        fmt.Printf("%s: Total=%d, Success=%d, Failed=%d, Unusable=%d\n",
            hsmID,
            stat.TotalAttestations,
            stat.SuccessfulCommits,
            stat.FailedCommits,
            stat.UnusableIndices)
    }
}
```

---

## Testing Specific Scenarios

### 1. Test Leader Failover

**Setup:**
1. Start all 3 nodes
2. Identify the leader (check `/leader` endpoint)

**Test:**
1. Terminate the leader node (Ctrl+C)
2. Wait 1-2 seconds
3. Check which node became the new leader:
   ```bash
   curl http://159.69.23.29:8080/leader
   curl http://159.69.23.30:8080/leader
   ```
4. Verify cluster still works:
   ```bash
   curl http://159.69.23.29:8080/health
   ```

**Expected:**
- New leader elected within 1-2 seconds
- Cluster continues operating
- All nodes can still send/receive messages

### 2. Test Hash Chain Integrity

**Using HSM Simulator:**

```go
sim := simulator.NewHSMSimulator("hsm-1", endpoints, genesisHash)

// Generate multiple attestations
for i := 0; i < 10; i++ {
    sim.GenerateAttestation(fmt.Sprintf("message-%d", i))
}

// Verify chain integrity
attestations := sim.GetAttestations()
// Check that each previous_hash matches hash of previous attestation
```

**Using API:**

```bash
# Get all log entries
curl http://159.69.23.29:8080/list

# Verify each entry's previous_hash matches previous entry's hash
```

### 3. Test Validation

**Test Invalid Attestation (Broken Hash Chain):**

```bash
# Create invalid attestation with wrong previous_hash
curl -X POST http://159.69.23.29:8080/propose \
  -H "Content-Type: application/json" \
  -d '{
    "attestation": {
      "attestation_response": {
        "data": {
          "value": "...",
          "encoding": "base64"
        },
        "signature": {
          "value": "...",
          "encoding": "base64"
        },
        "certificate": {
          "value": "...",
          "encoding": "pem"
        }
      }
    }
  }'
```

**Expected:**
- Validation error returned
- Attestation not committed
- Detailed error message explaining the issue

### 4. Test Concurrent HSMs

**Using Simulator Pool:**

```go
pool := simulator.NewHSMSimulatorPool(endpoints, genesisHash, 5)
err := pool.RunConcurrentAttestations(20, "load-test")
```

**Expected:**
- All attestations committed successfully
- No index reuse
- Hash chain remains intact
- Statistics show success for all HSMs

### 5. Test Discard Rule

**Scenario:** HSM generates attestation, but service rejects it

**Test:**
1. HSM generates attestation with invalid data
2. Service rejects it
3. HSM marks index as unusable
4. HSM skips that index for next attestation

**Verify:**
```go
protocol := client.NewHSMProtocol(client, genesisHash)
// Try to commit invalid attestation
committed, _, _, err := protocol.CommitAttestation(invalidAttestation, timeout)
if !committed {
    // Index should be marked unusable
    nextIndex := protocol.GetNextUsableIndex()
    // Should skip the unusable index
}
```

---

## Quick Test Checklist

### ✅ Basic Functionality

- [ ] All unit tests pass
- [ ] Service starts on all 3 nodes
- [ ] Leader is elected
- [ ] Can send messages via CLI
- [ ] Can list messages
- [ ] Health check works

### ✅ Raft Functionality

- [ ] Leader failover works
- [ ] Messages replicated to all nodes
- [ ] Cluster works with 2/3 nodes
- [ ] Bootstrap works correctly

### ✅ Hash Chain

- [ ] Genesis entry created correctly
- [ ] Hash chain links are valid
- [ ] Sequence numbers monotonic
- [ ] LMS indices monotonic

### ✅ Validation

- [ ] Invalid attestations rejected
- [ ] Valid attestations accepted
- [ ] Detailed error messages
- [ ] Hash chain validation works

### ✅ HSM Client

- [ ] Can sync state
- [ ] Can generate attestations
- [ ] Can commit attestations
- [ ] Discard rule works
- [ ] Unusable indices tracked

### ✅ HSM Simulator

- [ ] Single HSM works
- [ ] Multiple HSMs concurrent
- [ ] Statistics tracked correctly
- [ ] Errors collected

---

## Troubleshooting

### Tests Fail to Connect

**Problem:** Integration tests can't connect to service

**Solution:**
- Ensure service is running
- Check firewall rules
- Verify ports are accessible
- Check service logs

### Port Already in Use

**Problem:** `bind: address already in use`

**Solution:**
```bash
# Find process using port
lsof -i :7000
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different ports
./lms-service -id node1 -addr 127.0.0.1:7001 -api-port 8081
```

### Bootstrap Fails

**Problem:** `bootstrap only works on new clusters`

**Solution:**
```bash
# Clear Raft data
rm -rf raft-data-*

# Or use clear script
./clear_raft_data.sh
```

### No Leader Elected

**Problem:** Cluster has no leader

**Solution:**
- Check all nodes are running
- Check network connectivity
- Verify Raft ports are accessible
- Check logs for errors
- Reduce timeouts (already done in code)

---

## Performance Testing

### Load Test with Multiple HSMs

```go
pool := simulator.NewHSMSimulatorPool(endpoints, genesisHash, 10)
start := time.Now()
err := pool.RunConcurrentAttestations(100, "load-test")
duration := time.Since(start)

fmt.Printf("Committed %d attestations in %v\n", 10*100, duration)
```

### Measure Throughput

- Attestations per second
- Latency (commit time)
- Concurrent HSMs supported
- Cluster stability under load

---

## Next Steps

1. **Run all unit tests** to verify basic functionality
2. **Set up 3-node cluster** for manual testing
3. **Use HSM simulator** for load testing
4. **Test failover scenarios** to verify resilience
5. **Verify hash chain integrity** after operations

For more details, see:
- `TESTING_3_NODES.md` - Detailed 3-node setup
- `TEST_FAILOVER.md` - Failover testing guide
- `MODULE_6_SUMMARY.md` - HSM simulator documentation
