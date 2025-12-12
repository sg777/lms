# What Was Actually Built

## This is a PRODUCTION SYSTEM, not just tests!

### The Real System (Production Code):

1. **`lms-service`** - The actual service binary
   - Runs on your 3 nodes
   - Provides HTTP API on port 8080
   - Uses Raft for consensus
   - Stores attestations in hash chain

2. **HTTP API Endpoints** (`service/api.go`):
   - `GET /health` - Health check
   - `GET /leader` - Get current leader
   - `GET /latest-head` - Get latest attestation
   - `POST /propose` - Submit new attestation
   - `GET /list` - List all log entries
   - `POST /send` - Send simple message (for CLI)

3. **Hash-Chain FSM** (`fsm/hashchain_fsm.go`):
   - Stores attestations in cryptographically linked chain
   - Validates hash chain integrity
   - Enforces monotonicity (sequence numbers, LMS indices)
   - This is the ACTUAL storage engine

4. **HSM Client Library** (`client/`):
   - `hsm_client.go` - HTTP client for HSMs to connect
   - `protocol.go` - Complete protocol workflow
   - This is what REAL HSMs would use

5. **Validation Layer** (`validation/`):
   - Validates attestations before committing
   - Checks hash chain, monotonicity, structure
   - Rejects invalid attestations

### What's Just for Testing:

- `simulator/` - HSM simulator (ONLY for testing)
- `tests/` - Integration tests (ONLY for testing)

## How It Works (Production):

1. **Start the service** on 3 nodes:
   ```bash
   ./lms-service -id node1 -addr 159.69.23.29:7000
   ```

2. **HSM connects** using the client library:
   ```go
   client := client.NewHSMClient(endpoints, "hsm-1")
   protocol := client.NewHSMProtocol(client, genesisHash)
   ```

3. **HSM syncs state**:
   ```go
   protocol.SyncState() // Gets latest attestation from service
   ```

4. **HSM creates attestation**:
   ```go
   payload := protocol.CreateAttestationPayload(...)
   attestation := protocol.CreateAttestationResponse(...)
   ```

5. **HSM submits to service**:
   ```go
   protocol.CommitAttestation(attestation, timeout)
   ```

6. **Service validates and commits** via Raft

7. **All nodes replicate** the attestation

## The Simulator is Just a Helper

The `simulator/` package is ONLY for:
- Testing without real HSMs
- Load testing
- Development

**Real HSMs would use `client/` directly, not the simulator!**

## What You Can Do Right Now:

1. **Start the service**:
   ```bash
   ./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
   ```

2. **Use the HTTP API**:
   ```bash
   curl http://159.69.23.31:8080/health
   curl http://159.69.23.31:8080/latest-head
   ```

3. **Submit attestations** via API or client library

4. **The simulator is optional** - only if you want to test without real HSMs

## Summary:

- **Production System**: `main.go`, `service/`, `fsm/`, `client/`, `validation/`
- **Testing Only**: `simulator/`, `tests/`

The system is production-ready. The simulator is just a convenience for testing.

