# Complete List of Implemented Functionalities

## Table of Contents
1. [User Authentication & Management](#1-user-authentication--management)
2. [LMS Key Management](#2-lms-key-management)
3. [Message Signing & Verification](#3-message-signing--verification)
4. [Raft Chain Operations](#4-raft-chain-operations)
5. [Explorer Public Interface](#5-explorer-public-interface)
6. [Wallet Management](#6-wallet-management-chips)
7. [Blockchain Integration](#7-blockchain-integration-veruschips)
8. [Hash Chain Integrity](#8-hash-chain-integrity)
9. [Raft Consensus Features](#9-raft-consensus-features)
10. [Data Models & Storage](#10-data-models--storage)
11. [Frontend Features](#11-frontend-features)
12. [Key ID Generation](#12-key-id-generation)
13. [Bootstrap & Configuration](#13-bootstrap--configuration)
14. [Error Handling](#14-error-handling)
15. [Performance Optimizations](#15-performance-optimizations)

---

## 1. User Authentication & Management

### Registration & Login
- **User Registration**: `POST /api/auth/register`
  - Creates new user account with username and password
  - Returns JWT token on successful registration
- **User Login**: `POST /api/auth/login`
  - Authenticates users with credentials
  - Returns JWT token for authenticated sessions
- **Get Current User**: `GET /api/auth/me`
  - Returns authenticated user information
  - Requires valid JWT token

### Security
- JWT-based authentication for all protected endpoints
- Secure password hashing (bcrypt)
- Token-based session management
- User isolation (keys and wallets are user-specific)

---

## 2. LMS Key Management

### Key Operations
- **Generate Key**: `POST /api/my/generate`
  - Creates new LMS key with specified parameters
  - Automatic key ID generation: `{username}_{number}_{random}`
  - Commits genesis record (index 0) to Raft chain
  - Stores keys in encrypted local database
  
- **List Keys**: `GET /api/my/keys`
  - Lists all keys owned by authenticated user
  - Shows key parameters, current index, creation date
  - Displays blockchain enablement status
  
- **Export Key**: `POST /api/my/export`
  - Exports complete key data including private key
  - Secure JSON format for backup/migration
  
- **Import Key**: `POST /api/my/import`
  - Imports previously exported key
  - Validates key integrity before import
  
- **Delete Key**: `POST /api/my/delete`
  - Commits delete record to Raft chain before deletion
  - Cleans up associated blockchain settings
  - Preserves historical chain data

### Key Features
- No key ID reuse (incremental numbering with random suffix)
- Automatic index tracking
- Full lifecycle management (create, use, export, delete)

---

## 3. Message Signing & Verification

### Signing
- **Sign Message**: `POST /api/my/sign`
  - Signs message with specified LMS key
  - Auto-increments index and commits to Raft
  - Optional blockchain commit (if enabled for key)
  - Returns signature and updated index
  - Enforces index monotonicity

### Verification
- **Verify Signature**: `POST /api/my/verify`
  - Verifies LMS signature against message
  - Validates using public key
  - Returns verification status and details

### Security Guarantees
- Cryptographic chain integrity
- Index reuse prevention
- Automatic hash chain validation

---

## 4. Raft Chain Operations

### Write Operations
- **Commit Index**: `POST /commit_index`
  - Commits index update to Raft consensus
  - Used internally by HSM server
  - Ensures distributed agreement on index progression

### Read Operations
- **Get All Entries**: `GET /all_entries?limit=N`
  - Returns entries ordered by Raft log index (newest first)
  - Configurable limit (default 10, max 1000)
  - Used by explorer for recent commits view
  
- **Get Key Chain**: `GET /key/{key_id}/chain`
  - Returns complete hash chain for a key ID
  - Includes all entries across multiple pubkey hashes
  - Shows chain validity status
  
- **Get by Pubkey Hash**: `GET /pubkey_hash/{pubkey_hash}/index`
  - Retrieves index by public key hash
  - Used for chain verification
  
- **Get Latest Index**: `GET /key/{key_id}/index`
  - Returns most recent index for a key
  - Fast lookup for current state
  
- **List All Keys**: `GET /keys`
  - Returns all key IDs in the system
  - Used for discovery and enumeration

---

## 5. Explorer Public Interface

### Real-Time Monitoring
- **Recent Commits**: `GET /api/recent?limit=N`
  - Displays top 10 recent commits from Raft
  - Auto-refreshes every 5 seconds (silent mode)
  - Optimized to prevent UI flicker when idle
  - Intelligent incremental updates

### Search & Discovery
- **Search**: `GET /api/search?q={query}`
  - Search by key ID, hash, or index
  - Fuzzy matching support
  - Returns matching entries with context
  
- **Chain View**: `GET /api/chain/{key_id}`
  - Complete chain visualization
  - Handles multiple pubkey hashes per key ID
  - Shows all historical entries including deleted keys
  - Real-time integrity verification

### Analytics
- **Statistics**: `GET /api/stats`
  - Total keys and commits
  - Valid vs. broken chains count
  - Last commit timestamp
  - Filtered by bootstrap block height

### Blockchain View
- **Blockchain Commits**: `GET /api/blockchain`
  - Lists all blockchain commits from Verus identity
  - Shows canonical key IDs (normalized VDXF IDs)
  - Displays user-friendly key labels
  - Actual commit block heights from identity history
  - Transaction ID links
  - Bootstrap block height filtering (2742761)
  - Auto-refreshes every 10 seconds

---

## 6. Wallet Management (CHIPS)

### Wallet Operations
- **List Wallets**: `GET /api/my/wallet/list`
  - Lists all wallets owned by user
  - Shows address, balance, creation date
  
- **Create Wallet**: `POST /api/my/wallet/create`
  - Generates new CHIPS wallet address
  - Automatic registration with Verus node
  
- **Get Balance**: `GET /api/my/wallet/balance?address={addr}`
  - Retrieves current balance for specific address
  - Real-time query to blockchain node
  
- **Get Total Balance**: `GET /api/my/wallet/total-balance`
  - Calculates sum of all user wallet balances
  - Used for funding availability checks

### Balance Requirements
- **Blockchain Enable**: Minimum 0.0005 CHIPS
- **Signing Operations**: Minimum 0.0001 CHIPS
- **Actual Fees**: ~0.0001-0.0002 CHIPS (optimized)

---

## 7. Blockchain Integration (Verus/CHIPS)

### Per-Key Toggle
- **Toggle Blockchain**: `POST /api/my/key/blockchain/toggle`
  - Enable/disable blockchain commits for individual keys
  - On enable: Commits current latest index to blockchain
  - Validates wallet balance before enabling
  - Stores setting persistently
  
- **Blockchain Status**: `GET /api/my/key/blockchain/status`
  - Returns blockchain enablement for all user keys
  - Shows funding address per key

### Transaction Optimization
- **Optimized Commits**: Only sends single entry per transaction
  - **Before**: ~3.8KB transactions (0.0004 CHIPS fee)
  - **After**: ~1KB transactions (0.0001-0.0002 CHIPS fee)
  - **Savings**: 50-75% fee reduction

### Identity Integration
- Uses `getidentityhistory` for complete audit trail
- Append-only blockchain model (no data loss)
- Supports normalized VDXF IDs
- Automatic key ID label lookups

---

## 8. Hash Chain Integrity

### Validation Features
- **Chain Validation**: End-to-end hash chain verification
  - Validates hash linkage between entries
  - Special handling for genesis entries (index 0)
  - Detects broken chains automatically
  
- **Index Monotonicity**: Ensures strictly increasing indices
  - Prevents index reuse attacks
  - Enforces sequential progression
  
- **Hash Mismatch Detection**: Identifies chain corruption
  - Compares computed vs. stored hashes
  - Flags discrepancies for investigation

### Genesis Entry Handling
- Recognizes `GenesisHash` as valid previous hash for index 0
- Skips hash mismatch validation for genesis entries
- Maintains chain integrity from first entry forward

---

## 9. Raft Consensus Features

### High Availability
- **3-Node Cluster**: Tolerates 1 node failure
- **Automatic Leader Election**: Sub-second failover
- **Leader Forwarding**: Transparent request routing to leader
- **Bootstrap Mode**: Node 3 can initialize cluster
- **Health Checks**: `GET /health` endpoint

### Consensus Properties
- **Crash-Fault Tolerance**: Survives node crashes
- **Consistency**: Linearizable reads and writes
- **Persistence**: Durable storage in BoltDB
- **Log Replication**: Automatic synchronization

---

## 10. Data Models & Storage

### Key Index Entry Structure
```go
type KeyIndexEntry struct {
    KeyID        string  // User-friendly key identifier
    PubkeyHash   string  // SHA-256 hash of public key (base64)
    Index        uint64  // LMS index value
    PreviousHash string  // Hash chain linkage
    Hash         string  // Current entry hash
    Signature    string  // LMS signature
    PublicKey    string  // LMS public key
    RecordType   string  // create, sign, sync, delete
    RaftIndex    uint64  // Raft log index for ordering
}
```

### Record Types
- **create**: Initial key generation (index 0)
- **sign**: Signing operation (index increment)
- **sync**: Synchronization across nodes
- **delete**: Key deletion marker

### Storage
- **Raft Data**: `raft-data/nodeX/` directories
- **User DB**: SQLite database for users
- **Wallet DB**: SQLite database for CHIPS wallets
- **Blockchain Settings**: SQLite database for per-key settings

---

## 11. Frontend Features

### User Experience
- **Auto-Refresh**: Smart updates without UI flicker
  - Raft explorer: Every 5 seconds
  - Blockchain explorer: Every 10 seconds
  - Only updates when data actually changes
  
- **Copyable Error Dialog**: User-friendly error display
  - Modal with selectable text
  - Copy button for easy error reporting
  - Detailed error messages for debugging

### UI Components
- **Real-Time Updates**: WebSocket-like behavior via polling
- **Incremental Refresh**: Adds new data without full reload
- **Search Interface**: Instant search with type-ahead
- **Chain Visualization**: Interactive hash chain display
- **Responsive Design**: Mobile-friendly interface
- **Smooth Animations**: Highlight new entries with fade effects

---

## 12. Key ID Generation

### Format
```
{username}_{number}_{base64_random}
```

### Example
```
s1_1_drIHzA==
```

### Components
1. **Username**: Actual username from JWT claims
2. **Number**: Sequential number (no reuse after deletion)
3. **Random Suffix**: 32-bit random (4 bytes) base64-encoded

### Uniqueness Guarantees
- Finds max existing number for user
- Increments to ensure no collisions
- Random suffix adds extra entropy
- Prevents key ID reuse after deletion

---

## 13. Bootstrap & Configuration

### Bootstrap Block Height
- **Current**: 2742761
- **Purpose**: Filter old blockchain commits
- **Configuration**: `LMS_BOOTSTRAP_BLOCK_HEIGHT` environment variable
- **Default**: Hardcoded in `explorer/blockchain_config.go`

### Verus Identity
- **Name**: `sg777z.chips.vrsc@`
- **Purpose**: Stores LMS index attestations
- **Format**: ContentMultiMap with VDXF-normalized keys

### Endpoints
- **Raft Cluster**: 
  - Node 1: 159.69.23.29:8080
  - Node 2: 159.69.23.30:8080
  - Node 3: 159.69.23.31:8080
- **HSM Server**: 159.69.23.31:9090
- **Explorer**: Port 8081 (configurable)

---

## 14. Error Handling

### User-Facing Errors
- **Copyable Error Messages**: All errors shown in modal with copy button
- **Detailed Context**: Includes relevant parameters for debugging
- **Graceful Degradation**: Explorer works in read-only mode if HSM unavailable
- **Validation Errors**: Clear messages for invalid inputs

### Backend Logging
- **Comprehensive Logging**: All blockchain operations logged
- **Request Tracking**: Parameters and results logged
- **Error Context**: Full error details in logs
- **Performance Metrics**: Transaction sizes and fees logged

---

## 15. Performance Optimizations

### Transaction Size Optimization
- **Blockchain Commits**: Reduced from 3.8KB to ~1KB
- **Fee Reduction**: 50-75% cost savings
- **Method**: Send only new entry (blockchain is append-only)

### UI Optimizations
- **Smart Refresh**: Skip DOM updates when no changes
- **Order Preservation**: Direct use of API sorting (no re-sorting)
- **Flicker Prevention**: Compare by hash and RaftIndex before updating
- **Efficient Rendering**: Minimal DOM manipulation

### API Efficiency
- **Configurable Limits**: Prevents over-fetching
- **Silent Refresh Mode**: No unnecessary logging
- **Caching**: Key ID label caching for repeated lookups
- **Lazy Loading**: Data fetched only when needed

---

## Summary

This system provides a complete, production-ready solution for managing LMS hash-based signatures in a distributed environment with optional blockchain integration. Key highlights:

- **High Availability**: 3-node Raft cluster with automatic failover
- **Cost Optimized**: 50-75% reduction in blockchain transaction fees
- **User Friendly**: Modern web interface with real-time updates
- **Secure**: JWT authentication, encrypted storage, hash chain integrity
- **Scalable**: Efficient data structures and smart caching
- **Professional**: Comprehensive logging, error handling, and monitoring

For more details, see the [README](README.md) and [explorer documentation](explorer/README.md).
