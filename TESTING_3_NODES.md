# Testing on 3 Nodes

## Prerequisites

- 3 nodes with IPs:
  - Node 1: `159.69.23.29`
  - Node 2: `159.69.23.30`
  - Node 3: `159.69.23.31`
- Go 1.21+ installed on all nodes
- Network connectivity between nodes (port 7000 for Raft, port 8080 for API)
- Firewall rules allowing:
  - Port 7000 (Raft internal communication)
  - Port 8080 (HTTP API)

## Step 1: Build the Service

On each node, clone/build the service:

```bash
# If you haven't already, clone the repo
cd /root/lms
go build -o lms-service .
```

Or copy the binary to each node.

## Step 2: Start Node 1 (Bootstrap)

On Node 1 (`159.69.23.29`):

```bash
cd /root/lms
./lms-service \
  -id node1 \
  -addr 159.69.23.29:7000 \
  -api-port 8080 \
  -raft-port 7000 \
  -raft-dir ./raft-data \
  -bootstrap \
  -genesis-hash lms_genesis_hash_verifiable_state_chains
```

**Important**: Only bootstrap on the first node you start!

Expected output:
```
Starting Verifiable State Chains service
  Node ID: node1
  Raft Address: 159.69.23.29:7000
  API Port: 8080
  Raft Port: 7000
  Bootstrap: true
  Genesis Hash: lms_genesis_hash_verifiable_state_chains
Bootstrapping cluster...
Cluster bootstrapped successfully
Starting API server on :8080
Node node1 is now the leader
```

## Step 3: Start Node 2

On Node 2 (`159.69.23.30`):

```bash
cd /root/lms
./lms-service \
  -id node2 \
  -addr 159.69.23.30:7000 \
  -api-port 8080 \
  -raft-port 7000 \
  -raft-dir ./raft-data \
  -genesis-hash lms_genesis_hash_verifiable_state_chains
```

**Note**: No `-bootstrap` flag on subsequent nodes.

Expected output:
```
Starting Verifiable State Chains service
  Node ID: node2
  ...
Starting API server on :8080
```

## Step 4: Start Node 3

On Node 3 (`159.69.23.31`):

```bash
cd /root/lms
./lms-service \
  -id node3 \
  -addr 159.69.23.31:7000 \
  -api-port 8080 \
  -raft-port 7000 \
  -raft-dir ./raft-data \
  -genesis-hash lms_genesis_hash_verifiable_state_chains
```

## Step 5: Test the Cluster

### Test Health Endpoint

From any machine (can test all 3 nodes):

```bash
# Test Node 1
curl http://159.69.23.29:8080/health

# Test Node 2 (will forward to leader)
curl http://159.69.23.30:8080/health

# Test Node 3 (will forward to leader)
curl http://159.69.23.31:8080/health
```

Expected response:
```json
{
  "healthy": true,
  "leader": "node1",
  "is_leader": true,
  "term": 1
}
```

### Test Leader Endpoint

```bash
curl http://159.69.23.29:8080/leader
```

Expected response:
```json
{
  "leader_id": "node1",
  "leader_addr": "http://159.69.23.29:8080",
  "is_leader": true
}
```

### Test Latest Head (Initially Empty)

```bash
curl http://159.69.23.29:8080/latest-head
```

Expected response (no attestations yet):
```json
{
  "success": false,
  "error": "no attestations committed yet"
}
```

### Test Proposing an Attestation

Create a test attestation file `test_attestation.json`:

```json
{
  "attestation": {
    "attestation_response": {
      "policy": {
        "value": "LMS_ATTEST_POLICY",
        "algorithm": "PS256"
      },
      "data": {
        "value": "eyJwcmV2aW91c19oYXNoIjoibG1zX2dlbmVzaXNfaGFzaF92ZXJpZmlhYmxlX3N0YXRlX2NoYWlucyIsImxtc19pbmRleCI6MCwibWVzc2FnZV9zaWduZWQiOiJtZXNzYWdlX2hhc2hfMCIsInNlcXVlbmNlX251bWJlciI6MCwidGltZXN0YW1wIjoiMjAyNS0wMS0xNVQxMjowMDowMFoiLCJtZXRhZGF0YSI6ImdlbmVzaXMifQ==",
        "encoding": "base64"
      },
      "signature": {
        "value": "dGVzdF9zaWduYXR1cmU=",
        "encoding": "base64"
      },
      "certificate": {
        "value": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t...",
        "encoding": "pem"
      }
    }
  },
  "hsm_identifier": "hsm_test_1"
}
```

The base64 data decodes to:
```json
{
  "previous_hash": "lms_genesis_hash_verifiable_state_chains",
  "lms_index": 0,
  "message_signed": "message_hash_0",
  "sequence_number": 0,
  "timestamp": "2025-01-15T12:00:00Z",
  "metadata": "genesis"
}
```

Submit the attestation:

```bash
curl -X POST http://159.69.23.29:8080/propose \
  -H "Content-Type: application/json" \
  -d @test_attestation.json
```

Expected response:
```json
{
  "success": true,
  "committed": true,
  "raft_index": 1,
  "raft_term": 1,
  "message": "Attestation committed: Applied attestation: index=1, lms_index=0, sequence=0"
}
```

### Verify Latest Head

```bash
curl http://159.69.23.29:8080/latest-head
```

Should now return the committed attestation.

### Test Leader Forwarding

Try accessing a non-leader node:

```bash
# If node2 is not the leader, this will forward to leader
curl http://159.69.23.30:8080/latest-head
```

The request will be automatically forwarded to the leader.

## Step 6: Test Failover

1. Kill the leader node (Ctrl+C or `kill` command)
2. Wait a few seconds for leader election
3. Check which node is now the leader:

```bash
curl http://159.69.23.30:8080/leader
curl http://159.69.23.31:8080/leader
```

One of them should now be the leader.

4. Continue submitting attestations to any node - they'll forward to the new leader.

## Troubleshooting

### Port Already in Use
If you get "address already in use":
```bash
# Check what's using the port
sudo netstat -tlnp | grep 7000
sudo netstat -tlnp | grep 8080

# Kill the process or use different ports
```

### Nodes Can't Connect
- Check firewall rules
- Verify network connectivity: `ping 159.69.23.29`
- Check Raft logs for connection errors

### Bootstrap Issues
- Only bootstrap on the FIRST node
- If you need to re-bootstrap, delete `raft-data-*/raft.db` files on all nodes

### No Leader Elected
- Ensure at least 2 nodes are running (for 3-node cluster)
- Check that nodes can communicate on port 7000
- Verify all nodes have the same cluster configuration

## Next Steps

After successful testing, proceed to:
- **Module 4**: HSM Client Protocol
- **Module 5**: Validation Layer
- **Module 6**: HSM Simulator

