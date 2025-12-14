# Blockchain Fallback Integration

## Overview

The HSM protocol now supports optional blockchain fallback when the Raft service is unavailable. If a commit to Raft times out, the system can automatically fall back to committing the LMS index to the Verus/CHIPS blockchain.

## Architecture

**Primary Path (Normal Operation):**
```
HSM → Raft Service → Commit Success
```

**Fallback Path (When Raft Times Out):**
```
HSM → Raft Service → Timeout → Blockchain (Verus) → Commit Success
```

## Configuration

### 1. Create Blockchain Client

```go
import "github.com/verifiable-state-chains/lms/blockchain"

verusClient := blockchain.NewVerusClient(
    "http://127.0.0.1:22778",  // RPC URL
    "rpc-username",            // RPC username
    "rpc-password",            // RPC password
)
```

### 2. Compute Pubkey Hash

The pubkey_hash is computed from the LMS public key:

```go
import (
    "crypto/sha256"
    "encoding/hex"
)

func computePubkeyHashHex(lmsPublicKey []byte) string {
    hash := sha256.Sum256(lmsPublicKey)
    return hex.EncodeToString(hash[:])
}

pubkeyHashHex := computePubkeyHashHex(lmsPublicKey)
```

### 3. Configure HSM Protocol with Blockchain Fallback

```go
import "github.com/verifiable-state-chains/lms/client"

// Create blockchain config
blockchainConfig := &client.BlockchainConfig{
    Enabled:       true,                        // Enable fallback
    VerusClient:   verusClient,                // Blockchain client
    IdentityName:  "sg777z.chips.vrsc@",       // Verus identity
    PubkeyHashHex: pubkeyHashHex,              // LMS pubkey hash (hex)
}

// Create protocol with blockchain fallback
protocol := client.NewHSMProtocol(
    hsmClient,
    genesisHash,
    blockchainConfig,  // nil to disable blockchain fallback
)
```

### 4. Use Protocol as Normal

The protocol works exactly as before. If Raft times out, it will automatically attempt blockchain fallback:

```go
// Commit attestation (with timeout)
committed, raftIndex, raftTerm, err := protocol.CommitAttestation(attestation, 5*time.Second)

if committed {
    // Success! (either via Raft or blockchain)
    // If committed via blockchain, raftIndex and raftTerm will be 0
}
```

## Behavior

### When Raft Succeeds
- Normal operation
- Returns Raft index and term
- No blockchain interaction

### When Raft Times Out (with blockchain fallback enabled)
1. Attempts to commit to blockchain
2. If blockchain commit succeeds:
   - Updates protocol state (index, sequence number)
   - Returns success (raftIndex=0, raftTerm=0 indicates blockchain commit)
   - **Index is NOT marked as unusable** (commit succeeded via blockchain)
3. If blockchain commit also fails:
   - Applies Discard Rule (marks index as unusable)
   - Returns error

### When Blockchain Fallback is Disabled (nil config)
- Traditional behavior
- Raft timeout → Discard Rule
- Index marked as unusable

## Implementation Details

### Protocol State Tracking

When committed via blockchain:
- `LastRaftIndex = 0` (indicates blockchain commit)
- `LastRaftTerm = 0` (indicates blockchain commit)
- `CurrentLMSIndex` and `SequenceNumber` updated normally

### Pubkey Hash Format

- **Input**: LMS public key (byte array)
- **Computation**: SHA-256 hash
- **Output**: Hex string (64 characters for 32 bytes)
- **Storage**: Committed to Verus identity's contentmultimap

### Verus Identity Format

Each commit is stored in the identity's `contentmultimap`:
```json
{
  "<normalized_pubkey_hash>": [
    {
      "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "<lms_index>"
    }
  ]
}
```

Verus normalizes the pubkey_hash key using VDXF ID. Use `GetVDXFID()` to compute the normalized key for queries.

## Recovery and Synchronization

When Raft service restarts, it should:
1. Query Verus blockchain for commits made during outage
2. Reconcile blockchain commits with local log
3. Apply First-Confirmed-Wins rule for concurrent commits

This recovery mechanism is yet to be implemented in the Raft service layer.

## Testing

See `client/blockchain_example.go` for a complete example of HSM protocol with blockchain fallback enabled.

