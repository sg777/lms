# Verus Blockchain Integration

## Overview

The Verus blockchain integration provides an additional layer of redundancy and audit capability for LMS index attestations. This integration is **optional** and operates alongside the primary Raft-based consensus system.

## Architecture

### Dual-Layer Approach

```
┌──────────────────────────────────────────────────┐
│           Application Layer                      │
│         (HSM Server / Explorer)                  │
└───────────┬───────────────────┬──────────────────┘
            │                   │
    ┌───────▼──────┐    ┌──────▼──────────┐
    │ Raft Cluster │    │   Blockchain    │
    │  (Primary)   │    │   (Optional)    │
    │              │    │                 │
    │ • Required   │    │ • Per-key       │
    │ • Fast       │    │ • Audit trail   │
    │ • Consensus  │    │ • Public verify │
    └──────────────┘    └─────────────────┘
```

### Key Principles

1. **Raft is Primary**: All operations commit to Raft first
2. **Blockchain is Optional**: Per-key configuration
3. **Append-Only**: Blockchain preserves complete history
4. **Optimized Commits**: Minimal transaction sizes
5. **Public Auditability**: Anyone can verify blockchain history

## Implementation

### Identity-Based Commits

LMS indices are stored in a Verus identity's `contentmultimap` field:

```json
{
  "identity": "sg777z.chips.vrsc@",
  "contentmultimap": {
    "<normalized_pubkey_hash>": [
      {
        "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "<lms_index>"
      }
    ]
  }
}
```

### VDXF ID Normalization

Verus normalizes all keys in `contentmultimap` using VDXF (VerusID Extended Format):

```
Original:  0230f50cf3488efd2383b239b0ad550c5f9f0cfcb9834ad3b6c0dd2e83d9cac2
Normalized: iL6re1GZbypvZxssKbSNpHGJ...
```

Use the `getvdxfid` RPC to compute normalized IDs:
```bash
verus getvdxfid "0230f50cf3488efd2383b239b0ad550c5f9f0cfcb9834ad3b6c0dd2e83d9cac2"
```

## Transaction Optimization

### Problem (Before)

Original implementation preserved all existing `contentmultimap` entries in each update:

- **Transaction Size**: ~3.8KB (10 keys = 3.8KB)
- **Fee**: 0.0004 CHIPS per commit
- **Issue**: Grows linearly with number of keys

### Solution (Current)

Send **only the new entry** per transaction:

- **Transaction Size**: ~1KB (single entry)
- **Fee**: 0.0001-0.0002 CHIPS per commit
- **Savings**: 50-75% fee reduction

### Why This Works

Blockchain is **append-only** by nature:
1. Each `updateidentity` creates a new block transaction
2. History is preserved via `getidentityhistory` RPC
3. No need to re-send existing data

### Fee Calculation

Fees are based on transaction size:
- **< 1KB**: 0.0001 CHIPS
- **1-2KB**: 0.0002 CHIPS
- **2-3KB**: 0.0003 CHIPS
- **3-4KB**: 0.0004 CHIPS

## API Methods

### Write Operations

#### `UpdateIdentity`

Commits a single LMS index to the blockchain:

```go
txID, err := client.UpdateIdentity(
    identityName,   // "sg777z.chips.vrsc@"
    pubkeyHash,     // "0230f50cf3488efd..."
    lmsIndex,       // "42"
    fundingAddress, // "RNdtBgwRvPTvp2ooMZNrV75PEa9s4UvEV9"
)
```

**Parameters:**
- `identityName`: Verus identity name
- `pubkeyHash`: SHA-256 hash of LMS public key (hex)
- `lmsIndex`: LMS index value (string)
- `fundingAddress`: CHIPS address for transaction fees

**Returns:**
- Transaction ID on success
- Error with details on failure

**RPC Call:**
```json
{
  "method": "updateidentity",
  "params": [
    {
      "name": "sg777z.chips.vrsc@",
      "parent": "...",
      "systemid": "...",
      "contentmultimap": {
        "0230f50cf3488efd...": [
          {
            "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "42"
          }
        ]
      }
    },
    false,  // returntx
    false,  // tokenupdate
    0,      // feeoffer (0 = standard fee)
    "RNdtBgwRvPTvp2ooMZNrV75PEa9s4UvEV9"  // sourceoffunds
  ]
}
```

#### `CommitLMSIndexWithPubkeyHash`

Helper method that pre-computes normalized VDXF ID:

```go
normalizedKeyID, txID, err := client.CommitLMSIndexWithPubkeyHash(
    identityName,
    pubkeyHashHex,
    lmsIndex,
    fundingAddress,
)
```

**Returns:**
- `normalizedKeyID`: VDXF-normalized key ID
- `txID`: Transaction ID
- `err`: Error if any

### Read Operations

#### `GetIdentity`

Returns the **current state** of an identity:

```go
identity, err := client.GetIdentity("sg777z.chips.vrsc@")
```

**Response:**
```json
{
  "fullyqualifiedname": "sg777z.chips.vrsc@",
  "identity": {
    "name": "sg777z.chips.vrsc@",
    "contentmultimap": {
      "iL6re1GZbypvZxssKbSNpHGJ...": [
        {
          "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "42"
        }
      ]
    }
  },
  "blockheight": 2743051,
  "txid": "60da07c6095eddc7328d6ed6f81f3b1889f8f137d76da0b93060e68ce5bf86db"
}
```

**Note:** Shows only the latest state (most recent entries per key).

#### `GetIdentityHistory`

Returns **complete history** of all identity updates:

```go
history, err := client.GetIdentityHistory(
    identityName,
    heightStart,  // 0 = from genesis
    heightEnd,    // 0 = to current height
)
```

**Response:**
```json
{
  "fullyqualifiedname": "sg777z.chips.vrsc@",
  "blockheight": 2743051,
  "txid": "60da07c6...",
  "history": [
    {
      "identity": {
        "name": "sg777z.chips.vrsc@",
        "contentmultimap": {
          "iL6re1GZbypvZxssKbSNpHGJ...": [
            {
              "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "0"
            }
          ]
        }
      },
      "height": 2741812,
      "blockhash": "0000000...",
      "output": {
        "txid": "4c532bc752e3977545046c64834ab92aba87c1d8189ba94d909fd6193e887b49",
        "voutnum": 0
      }
    },
    // ... more entries
  ]
}
```

**Critical for:**
- Retrieving all historical commits
- Finding actual commit block heights
- Audit trail reconstruction
- Delete record discovery

#### `QueryAttestationCommits`

Queries all commits from history (not current state):

```go
commits, err := client.QueryAttestationCommits(
    identityName,
    keyID,  // optional filter
)
```

**Implementation:** Uses `GetIdentityHistory` internally, processes all entries, and deduplicates by `(keyID, lmsIndex)`.

## Bootstrap Block Height

### Purpose

Filter out commits from before the system was operational:

```go
// Default: 2742761
bootstrapHeight := getBootstrapBlockHeight()
```

### Configuration

1. **Environment Variable:**
   ```bash
   export LMS_BOOTSTRAP_BLOCK_HEIGHT=2742761
   ```

2. **Code (Hardcoded Default):**
   ```go
   // explorer/blockchain_config.go
   func getBootstrapBlockHeight() int64 {
       return 2742761
   }
   ```

### Usage

```go
// Filter commits by block height
if commit.BlockHeight >= bootstrapHeight {
    // Include in results
}
```

## Per-Key Blockchain Toggle

### Enable Blockchain for a Key

```bash
POST /api/my/key/blockchain/toggle
{
  "key_id": "s1_1_drIHzA==",
  "enable": true
}
```

**Process:**
1. Validate wallet balance (minimum 0.0005 CHIPS)
2. Query Raft for latest index
3. Commit current index to blockchain
4. Store setting in database

**Fee Requirement:**
- **Minimum Balance**: 0.0005 CHIPS
- **Actual Fee**: ~0.0001-0.0002 CHIPS per commit
- **Buffer**: Extra for transaction overhead

### Disable Blockchain for a Key

```bash
POST /api/my/key/blockchain/toggle
{
  "key_id": "s1_1_drIHzA==",
  "enable": false
}
```

**Effect:** Future commits for this key will **not** go to blockchain (Raft only).

**Note:** Historical blockchain commits remain immutable.

## Blockchain Explorer

### View All Commits

```bash
GET /api/blockchain
```

**Features:**
- Lists all commits from identity history
- Shows actual commit block heights
- Displays canonical key IDs (VDXF normalized)
- Provides user-friendly key labels
- Filters by bootstrap block height
- Auto-refreshes every 10 seconds

**Response:**
```json
{
  "success": true,
  "identity": "sg777z.chips.vrsc@",
  "block_height": 2743051,
  "commits": [
    {
      "key_id": "iL6re1GZbypvZxssKbSNpHGJ...",
      "key_id_label": "s1_1_drIHzA==",
      "lms_index": "42",
      "block_height": 2742850,
      "txid": "60da07c6..."
    }
  ]
}
```

## Best Practices

### 1. Use Raft as Primary

Always commit to Raft first:
```go
// Commit to Raft (required)
err := commitToRaft(entry)
if err != nil {
    return err
}

// Commit to blockchain (optional, if enabled)
if blockchainEnabled {
    _ = commitToBlockchain(entry)  // Don't fail if blockchain errors
}
```

### 2. Handle Blockchain Errors Gracefully

Blockchain should not block operations:
```go
if err := commitToBlockchain(entry); err != nil {
    log.Printf("[WARNING] Blockchain commit failed: %v", err)
    // Continue - Raft commit succeeded
}
```

### 3. Fund Wallets Proactively

Maintain wallet balance:
- Check balance before enabling blockchain
- Keep buffer for multiple commits
- Monitor balance in Explorer

### 4. Use History for Audit

Query history for complete records:
```go
// Don't rely on current state
history := client.GetIdentityHistory(identity, 0, 0)

// Process all entries
for _, entry := range history.History {
    // Extract commits from each historical state
}
```

### 5. Leverage Bootstrap Height

Filter old/test data:
```go
if commit.BlockHeight >= bootstrapHeight {
    // Production data
}
```

## Troubleshooting

### "Insufficient balance" Error

**Symptoms:** Blockchain enable fails with balance error

**Solutions:**
1. Fund wallet: Send CHIPS to wallet address
2. Check balance: `/api/my/wallet/total-balance`
3. Verify minimum: Need 0.0005 CHIPS

### Transaction Size Too Large

**Symptoms:** RPC error code -4, message about transaction size

**Cause:** Identity has too many entries (shouldn't happen with optimization)

**Solution:**
- Verify optimization is active (only sends single entry)
- Check logs for entry count
- Contact support if persists

### Missing Blockchain Commits

**Symptoms:** Commits not appearing in blockchain explorer

**Possible Causes:**
1. **Below Bootstrap Height**: Filtered out
2. **Not Enabled**: Blockchain toggle off for that key
3. **Transaction Pending**: Wait for block confirmation
4. **Insufficient Balance**: Transaction never sent

**Debug:**
```bash
# Check if blockchain is enabled
GET /api/my/key/blockchain/status

# Check wallet balance
GET /api/my/wallet/total-balance

# Check logs
tail -f explorer.log | grep BLOCKCHAIN
```

### VDXF ID Mismatch

**Symptoms:** Can't find commit by pubkey hash

**Cause:** Searching with non-normalized ID

**Solution:**
```bash
# Normalize first
normalized=$(verus getvdxfid "0230f50cf3488efd...")

# Then search
GET /api/blockchain?key_id=$normalized
```

## CLI Tool

### Usage

```bash
./verus-cli [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-rpc-url` | Verus RPC URL | `http://127.0.0.1:22778` |
| `-rpc-user` | RPC username | (from config) |
| `-rpc-password` | RPC password | (from config) |
| `-identity` | Identity to use | `sg777z.chips.vrsc@` |
| `-action` | Action: `info`, `commit`, `query` | (required) |
| `-key-id` | LMS key ID | (for commit/query) |
| `-lms-index` | LMS index | (for commit) |

### Examples

**View identity info:**
```bash
./verus-cli -action=info
```

**Commit an index:**
```bash
./verus-cli -action=commit -key-id="s1_1" -lms-index="42"
```

**Query commits:**
```bash
# All commits
./verus-cli -action=query

# Specific key
./verus-cli -action=query -key-id="s1_1"
```

## Performance Metrics

### Transaction Fees

| Entries in Identity | Old Fee | New Fee | Savings |
|---------------------|---------|---------|---------|
| 1 key               | 0.0001  | 0.0001  | 0%      |
| 5 keys              | 0.0002  | 0.0001  | 50%     |
| 10 keys             | 0.0004  | 0.0001  | 75%     |
| 20 keys             | 0.0008  | 0.0001  | 87.5%   |

### Latency

| Operation | Latency |
|-----------|---------|
| `UpdateIdentity` | 2-5 seconds (block confirmation) |
| `GetIdentity` | < 100ms |
| `GetIdentityHistory` | < 500ms (100 entries) |

## Future Enhancements

### Planned Features

1. **Automatic Fee Estimation**: Query network for optimal fees
2. **Batch Commits**: Combine multiple updates in single transaction
3. **SPV Support**: Light client verification
4. **Multi-Identity**: Support multiple identities per system

### Under Consideration

- **Cross-Chain**: Support for other Verus-based chains
- **Privacy**: zk-SNARK proofs for private commits
- **Incentives**: Reward mechanism for validators

## References

- **Verus Docs**: [verus.io/documentation](https://verus.io/documentation)
- **VerusID**: [wiki.verus.io/verusid](https://wiki.verus.io/verusid)
- **VDXF Spec**: [wiki.verus.io/vdxf](https://wiki.verus.io/vdxf)
- **RPC API**: [wiki.verus.io/rpc](https://wiki.verus.io/rpc)

## Support

For blockchain-specific issues:
1. Check Verus node is synced: `verus getinfo`
2. Verify RPC connectivity: `curl -u user:pass http://localhost:22778`
3. Review explorer logs: `tail -f explorer.log`
4. Consult Verus documentation

---

**Last Updated**: December 2024  
**Optimization Level**: Production Ready (50-75% fee reduction achieved)
