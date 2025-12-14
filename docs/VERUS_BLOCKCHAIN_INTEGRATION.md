# Verus Blockchain Integration

## Overview

The Verus blockchain integration allows LMS index attestations to be committed to the Verus/CHIPS blockchain via identity updates. This provides a fallback mechanism when the primary Raft service is unavailable.

## Architecture

- **Identity-based commits**: LMS indices are stored in the identity's `contentmultimap` field
- **Format**: Each commit stores `key_id -> [{identifier: lms_index}]`
- **Identity**: `sg777z.chips.vrsc@` (CHIPS chain on Verus)

## CLI Tool

A CLI tool (`verus-cli`) is available for testing and manual operations:

### Usage

```bash
./verus-cli [flags]
```

### Flags

- `-rpc-url`: Verus RPC URL (default: `http://127.0.0.1:22778`)
- `-rpc-user`: RPC username (default: from config)
- `-rpc-password`: RPC password (default: from config)
- `-identity`: Identity to use (default: `sg777z.chips.vrsc@`)
- `-action`: Action to perform: `info`, `commit`, or `query`
- `-key-id`: LMS key ID (required for commit/query)
- `-lms-index`: LMS index to commit (required for commit)

### Examples

**View identity information:**
```bash
./verus-cli -action=info
```

**Commit an LMS index:**
```bash
./verus-cli -action=commit -key-id="user_abc_key_1" -lms-index="42"
```

**Query all commits:**
```bash
./verus-cli -action=query
```

**Query commits for a specific key:**
```bash
./verus-cli -action=query -key-id="user_abc_key_1"
```

## Implementation Status

✅ **Completed:**
- Verus RPC client (`blockchain/verus_client.go`)
- Get identity information
- Update identity with LMS index commits
- Query identity for committed indices
- CLI tool for testing

⏳ **In Progress:**
- HSM protocol integration (blockchain fallback on timeout)
- First-Confirmed-Wins rule implementation
- Recovery mechanism (query blockchain on Raft restart)

## Data Format

The LMS index is stored in the identity's `contentmultimap` with the following structure:

```json
{
  "key_id": [
    {
      "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "<lms_index>"
    }
  ]
}
```

**Note:** Verus may normalize or hash the `key_id` when storing, so queries should use the actual stored key format.

## RPC Configuration

The RPC credentials are stored in:
```
~/.verus/pbaas/f315367528394674d45277e369629605a1c3ce9f/f315367528394674d45277e369629605a1c3ce9f.conf
```

- RPC URL: `http://127.0.0.1:22778`
- Username: `user1172159772`
- Password: `pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da`

## Next Steps

1. **HSM Protocol Integration**: Add blockchain fallback when Raft timeout occurs
2. **Recovery Mechanism**: Query blockchain on Raft service restart
3. **First-Confirmed-Wins**: Implement conflict resolution for concurrent commits
4. **Explorer UI**: Add blockchain commit status to the explorer interface

