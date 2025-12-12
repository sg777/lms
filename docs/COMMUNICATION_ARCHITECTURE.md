# Communication Architecture

## Current State

### ✅ Raft-to-Raft Communication (IMPLEMENTED)

The existing code in `/root/raft/main.go` already implements **Raft internal communication**:

```go
// Line 97: TCP Transport for Raft consensus
transport, err := raft.NewTCPTransport("0.0.0.0:7000", addr, 3, 10*time.Second, os.Stderr)
```

**What this does:**
- Nodes communicate on **port 7000** via TCP
- Handles leader election
- Replicates log entries between nodes
- Maintains consensus (quorum-based)

**How it works:**
1. Node 1, 2, 3 all listen on port 7000
2. Raft library handles all internal communication automatically
3. Leader replicates entries to followers
4. Followers acknowledge and commit

### ❌ HSM-to-Service Communication (NOT IMPLEMENTED YET)

**What's missing:**
- HTTP API server for HSM clients
- Leader forwarding logic
- Service layer that wraps Raft

**What needs to be built (Module 2):**

```
HSM Client                    Service Node (Non-Leader)          Service Node (Leader)          Raft Cluster
    |                                  |                                |                            |
    |-- HTTP GET /latest-head -------->|                                |                            |
    |                                  |-- Check: Am I leader?         |                            |
    |                                  |-- NO: Forward to leader ------>|                            |
    |                                  |                                |-- Query Raft FSM ---------->|
    |                                  |                                |<-- Return latest entry -----|
    |<-- Return latest attestation ----|<-- Forward response ----------|                            |
    |                                  |                                |                            |
    |-- HTTP POST /propose ----------->|                                |                            |
    |                                  |-- Forward to leader ---------->|                            |
    |                                  |                                |-- Apply to Raft ----------->|
    |                                  |                                |<-- Committed --------------|
    |<-- Success response -------------|<-- Forward response ----------|                            |
```

## Communication Layers

### Layer 1: Raft Internal (Port 7000) ✅
- **Protocol**: TCP (HashiCorp Raft protocol)
- **Purpose**: Consensus, replication, leader election
- **Status**: Working in `/root/raft/main.go`

### Layer 2: HSM-to-Service (Port 8080 - to be implemented) ❌
- **Protocol**: HTTP/REST
- **Purpose**: HSM clients submit attestations, query state
- **Status**: **Module 2 will implement this**

## What Module 2 Will Add

1. **HTTP API Server** (port 8080)
   - `/health` - Health check
   - `/leader` - Get leader info
   - `/latest-head` - Get latest attestation
   - `/propose` - Submit new attestation

2. **Leader Forwarding**
   - Non-leader nodes detect they're not leader
   - Automatically forward requests to leader
   - Return leader address to client (for direct connection)

3. **Service Wrapper**
   - Wraps Raft instance
   - Provides clean API interface
   - Handles request/response conversion

## Testing Communication

### Test Raft Internal Communication (Current)

```bash
# On Node 1 (159.69.23.29)
cd /root/raft
go run main.go -id node1 -addr 159.69.23.29:7000 -bootstrap

# On Node 2 (159.69.23.30)
go run main.go -id node2 -addr 159.69.23.30:7000

# On Node 3 (159.69.23.31)
go run main.go -id node3 -addr 159.69.23.31:7000
```

You should see:
- Leader election happening
- Log entries being replicated
- All nodes seeing the same logs

### Test HSM-to-Service Communication (After Module 2)

```bash
# HSM client will be able to:
curl http://159.69.23.29:8080/health
curl http://159.69.23.29:8080/latest-head
curl -X POST http://159.69.23.29:8080/propose -d @attestation.json
```

Even if you hit node2 or node3, they'll forward to the leader automatically.

