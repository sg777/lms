# Module 4: HSM Client Protocol ✅

## Status: COMPLETE

## What Was Implemented

### 1. HSM Client Library (`client/hsm_client.go`)

**HSMClient**: Complete client for HSM partitions to interact with the service

**Features**:
- **Automatic Leader Discovery**: Connects to any service node, automatically forwards to leader
- **Multi-Endpoint Support**: Tries multiple endpoints until one succeeds
- **HTTP Client**: Configurable timeout for requests
- **Error Handling**: Comprehensive error handling with detailed messages

**Methods**:
- `GetLatestHead()`: Fetches latest attestation head from service
- `ProposeAttestation()`: Submits attestation for commitment
- `HealthCheck()`: Checks service health
- `GetLeaderInfo()`: Gets current leader information

### 2. Protocol State Management (`client/protocol.go`)

**ProtocolState**: Tracks HSM's protocol state
- Current LMS index
- Sequence number (monotonically increasing)
- Last committed attestation
- Last Raft index/term seen
- Unusable indices (Discard Rule)

**HSMProtocol**: Complete protocol workflow implementation

**Features**:
- **State Synchronization**: `SyncState()` fetches latest state from service
- **Attestation Construction**: `CreateAttestationPayload()` builds correct payload with `previous_hash`
- **Attestation Response**: `CreateAttestationResponse()` creates complete attestation structure
- **Commit Workflow**: `CommitAttestation()` submits and handles Discard Rule
- **Index Management**: `GetNextUsableIndex()` skips unusable indices
- **Complete Workflow**: `CompleteWorkflow()` combines all steps

### 3. Discard Rule Implementation

**Critical Security Feature**: If attestation is rejected or times out:
- Index is automatically marked as unusable
- HSM must discard the LMS signature for that index
- Index can never be reused

### 4. Helper Functions

- `ComputeGenesisHash()`: Computes genesis hash from LMS public key + system bundle
- `IsIndexUsable()`: Checks if an index is still usable

## Key Features

### Automatic Leader Forwarding

The client automatically handles leader forwarding:
```go
// Connect to any node - will auto-forward to leader
client := NewHSMClient([]string{
    "http://159.69.23.29:8080",  // Any node
    "http://159.69.23.30:8080",  // Will try these
    "http://159.69.23.31:8080",  // Until one works
}, "hsm-001")
```

### Complete Workflow

```go
// Step 1: Create client and protocol
client := NewHSMClient(endpoints, "hsm-001")
protocol := NewHSMProtocol(client, genesisHash)

// Step 2: Sync state
protocol.SyncState()

// Step 3: Get next usable index
nextIndex := protocol.GetNextUsableIndex()

// Step 4: Create attestation payload
payload, _ := protocol.CreateAttestationPayload(
    nextIndex,
    messageHash,
    timestamp,
    metadata,
)

// Step 5: Create attestation response (HSM signs this)
attestation, _ := protocol.CreateAttestationResponse(
    payload,
    "LMS_ATTEST_POLICY",
    "PS256",
    signature,
    certificate,
)

// Step 6: Commit attestation (handles Discard Rule automatically)
committed, raftIndex, raftTerm, err := protocol.CommitAttestation(
    attestation,
    5*time.Second,
)

if !committed {
    // Index automatically marked as unusable
    // HSM must discard signature
}
```

### Convenience Method

```go
// All steps combined
committed, raftIndex, raftTerm, err := protocol.CompleteWorkflow(
    messageHash,
    "LMS_ATTEST_POLICY",
    "PS256",
    signature,
    certificate,
    5*time.Second,
)
```

## Testing

### Unit Tests

```bash
cd /root/lms
go test ./client -v
```

All tests pass ✅:
- `TestHSMClient_NewHSMClient`
- `TestProtocolState_NewProtocolState`
- `TestHSMProtocol_NewHSMProtocol`
- `TestHSMProtocol_CreateAttestationPayload_Genesis`
- `TestHSMProtocol_CreateAttestationPayload_WithPrevious`
- `TestHSMProtocol_CreateAttestationResponse`
- `TestHSMProtocol_IsIndexUsable`
- `TestHSMProtocol_GetNextUsableIndex`
- `TestComputeGenesisHash`
- `TestHSMProtocol_DiscardRule`

## Files Created

- `client/hsm_client.go` - HSM client library
- `client/protocol.go` - Protocol workflow and state management
- `client/client_test.go` - Comprehensive unit tests
- `client/example.go` - Example usage code

## Integration Points

### With Service Layer
- Uses existing API endpoints: `/latest-head`, `/propose`, `/health`, `/leader`
- Automatically handles leader forwarding
- Compatible with existing Raft cluster

### With FSM
- Works with `HashChainFSM` validation
- Respects hash chain integrity rules
- Handles genesis entry correctly

## Next Steps: Module 5

Module 5 will add:
- **Validation Layer**: Strict validation (hash verification, index monotonicity, signature checks)
- **Security Checks**: Cryptographic verification of attestations
- **Enhanced Error Handling**: Detailed validation error messages

## Usage Example

See `client/example.go` for complete usage examples showing:
- Basic workflow
- Complete workflow convenience method
- Error handling
- Discard Rule behavior

