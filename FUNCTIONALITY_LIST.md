# Complete List of Implemented Functionalities

## 1. User Authentication & Management
- **User Registration**: `/api/auth/register` - Register new users
- **User Login**: `/api/auth/login` - Authenticate users, returns JWT token
- **Get Current User**: `/api/auth/me` - Get authenticated user info
- **JWT Token Authentication**: All authenticated endpoints require valid JWT token

## 2. LMS Key Management (HSM Server)
- **Generate Key**: `/api/my/generate` - Generate new LMS key with parameters
  - Supports username-based key ID format: `{username}_{number}_{random_suffix}`
  - Stores keys in local database
  - Commits create record (index 0) to Raft chain
- **List Keys**: `/api/my/keys` - List all keys for authenticated user
- **Export Key**: `/api/my/export` - Export key (including private key data)
- **Import Key**: `/api/my/import` - Import previously exported key
- **Delete Key**: `/api/my/delete` - Delete a key
  - Commits delete record to Raft chain before deletion
  - Cleans up blockchain settings for deleted key

## 3. Message Signing & Verification
- **Sign Message**: `/api/my/sign` - Sign a message with specified key
  - Commits sign record to Raft chain (index increment)
  - Optionally commits to blockchain if enabled for the key
- **Verify Signature**: `/api/my/verify` - Verify a signature against message and public key

## 4. Raft Chain Operations
- **Commit Index**: `/commit_index` - Commit index update to Raft (used by HSM server)
- **Get All Entries**: `/all_entries?limit=N` - Get all entries ordered by Raft log index (newest first)
- **Get Key Chain**: `/key/{key_id}/chain` - Get full hash chain for a key ID
- **Get Pubkey Hash Chain**: `/pubkey_hash/{pubkey_hash}/index` - Get index by pubkey_hash
- **Get Latest Index**: `/key/{key_id}/index` - Get latest index for a key ID
- **List All Keys**: `/keys` - Get all key IDs in the system

## 5. Explorer Public Interface
- **Recent Commits**: `/api/recent?limit=N` - Get recent commits from Raft (top 10, refreshes every 5 seconds)
- **Search**: `/api/search?q={query}` - Search by key_id, hash, or index
- **Statistics**: `/api/stats` - Get overall statistics (total keys, commits, valid/broken chains)
- **Chain View**: `/api/chain/{key_id}` - Get full chain for a key ID
  - Handles multiple pubkey_hashes for same key_id
  - Shows all historical entries across all pubkey_hashes
- **Blockchain Commits**: `/api/blockchain` - Get all blockchain commits
  - Filters by bootstrap block height
  - Shows actual commit block heights from identity history
  - Displays canonical key IDs and user-friendly labels

## 6. Wallet Management (CHIPS)
- **List Wallets**: `/api/my/wallet/list` - List all wallets for user
- **Create Wallet**: `/api/my/wallet/create` - Create new CHIPS wallet address
- **Get Balance**: `/api/my/wallet/balance?address={addr}` - Get balance for specific address
- **Get Total Balance**: `/api/my/wallet/total-balance` - Get total balance across all user's wallets

## 7. Blockchain Integration (Verus/CHIPS)
- **Toggle Blockchain**: `/api/my/key/blockchain/toggle` - Enable/disable blockchain for a key
  - When enabling: Commits current latest index to blockchain
  - Requires wallet with sufficient balance (0.0001 CHIPS minimum)
  - Stores setting in database
- **Blockchain Status**: `/api/my/key/blockchain/status` - Get blockchain enablement status for all keys
- **Blockchain Explorer**: View all blockchain commits with:
  - Canonical Key IDs (normalized VDXF IDs)
  - User-friendly Key ID labels
  - Actual commit block heights (from identity history)
  - Transaction IDs
  - Bootstrap block height filtering

## 8. Hash Chain Integrity
- **Chain Validation**: Validates hash chain integrity
  - Ensures each entry's hash links to previous entry
  - Special handling for genesis entries (index 0)
  - Detects broken chains
- **Index Monotonicity**: Ensures indices are always incrementing
- **Hash Mismatch Detection**: Detects when hash chain is broken

## 9. Raft Consensus Features
- **Leader Election**: Automatic leader election in 3-node cluster
- **Leader Forwarding**: Non-leader nodes forward requests to leader
- **Fault Tolerance**: Can tolerate 1 node failure (out of 3)
- **Bootstrap Mode**: Node 3 can bootstrap cluster
- **Health Checks**: `/health` endpoint shows cluster health and leader status

## 10. Data Models & Storage
- **Key Index Entry**: Stores key_id, pubkey_hash, index, previous_hash, hash, signature, public_key, record_type, raft_index
- **Record Types**: create, sign, sync, delete
- **Raft Index Tracking**: Each entry tracks its Raft log index for chronological ordering
- **Persistent Storage**: Raft stores data in `raft-data/nodeX/` directories

## 11. Frontend Features
- **Copyable Error Dialog**: Error messages displayed in modal with copyable textarea
- **Real-time Updates**: Recent commits refresh every 5 seconds
- **Incremental Refresh**: Only adds new commits, doesn't reload entire list
- **Search Interface**: Search by key ID, hash, or index
- **Chain Visualization**: View full hash chain for any key
- **Blockchain Explorer**: View all blockchain commits with filtering

## 12. Key ID Generation
- **Format**: `{username}_{number}_{base64_random_32bits}`
- **Example**: `s1_1_drIHzA==`
- **Uniqueness**: Ensures no key ID reuse by incrementing from max existing number
- **Random Suffix**: 32-bit random number (4 bytes) encoded as base64 for uniqueness

## 13. Bootstrap & Configuration
- **Bootstrap Block Height**: Hardcoded to 2737418 (configurable via `LMS_BOOTSTRAP_BLOCK_HEIGHT`)
- **Identity Name**: Configurable Verus identity (default: `sg777z.chips.vrsc@`)
- **Raft Endpoints**: Configurable Raft cluster endpoints
- **Database Paths**: Separate databases for users, wallets, key blockchain settings

## 14. Error Handling
- **Copyable Error Messages**: Errors displayed in modal with selectable text
- **Graceful Degradation**: Explorer works even if HSM server is down (read-only mode)
- **Validation Errors**: Clear error messages for validation failures
- **Blockchain Errors**: Detailed error messages for blockchain operation failures


## 1. User Authentication & Management
- **User Registration**: `/api/auth/register` - Register new users
- **User Login**: `/api/auth/login` - Authenticate users, returns JWT token
- **Get Current User**: `/api/auth/me` - Get authenticated user info
- **JWT Token Authentication**: All authenticated endpoints require valid JWT token

## 2. LMS Key Management (HSM Server)
- **Generate Key**: `/api/my/generate` - Generate new LMS key with parameters
  - Supports username-based key ID format: `{username}_{number}_{random_suffix}`
  - Stores keys in local database
  - Commits create record (index 0) to Raft chain
- **List Keys**: `/api/my/keys` - List all keys for authenticated user
- **Export Key**: `/api/my/export` - Export key (including private key data)
- **Import Key**: `/api/my/import` - Import previously exported key
- **Delete Key**: `/api/my/delete` - Delete a key
  - Commits delete record to Raft chain before deletion
  - Cleans up blockchain settings for deleted key

## 3. Message Signing & Verification
- **Sign Message**: `/api/my/sign` - Sign a message with specified key
  - Commits sign record to Raft chain (index increment)
  - Optionally commits to blockchain if enabled for the key
- **Verify Signature**: `/api/my/verify` - Verify a signature against message and public key

## 4. Raft Chain Operations
- **Commit Index**: `/commit_index` - Commit index update to Raft (used by HSM server)
- **Get All Entries**: `/all_entries?limit=N` - Get all entries ordered by Raft log index (newest first)
- **Get Key Chain**: `/key/{key_id}/chain` - Get full hash chain for a key ID
- **Get Pubkey Hash Chain**: `/pubkey_hash/{pubkey_hash}/index` - Get index by pubkey_hash
- **Get Latest Index**: `/key/{key_id}/index` - Get latest index for a key ID
- **List All Keys**: `/keys` - Get all key IDs in the system

## 5. Explorer Public Interface
- **Recent Commits**: `/api/recent?limit=N` - Get recent commits from Raft (top 10, refreshes every 5 seconds)
- **Search**: `/api/search?q={query}` - Search by key_id, hash, or index
- **Statistics**: `/api/stats` - Get overall statistics (total keys, commits, valid/broken chains)
- **Chain View**: `/api/chain/{key_id}` - Get full chain for a key ID
  - Handles multiple pubkey_hashes for same key_id
  - Shows all historical entries across all pubkey_hashes
- **Blockchain Commits**: `/api/blockchain` - Get all blockchain commits
  - Filters by bootstrap block height
  - Shows actual commit block heights from identity history
  - Displays canonical key IDs and user-friendly labels

## 6. Wallet Management (CHIPS)
- **List Wallets**: `/api/my/wallet/list` - List all wallets for user
- **Create Wallet**: `/api/my/wallet/create` - Create new CHIPS wallet address
- **Get Balance**: `/api/my/wallet/balance?address={addr}` - Get balance for specific address
- **Get Total Balance**: `/api/my/wallet/total-balance` - Get total balance across all user's wallets

## 7. Blockchain Integration (Verus/CHIPS)
- **Toggle Blockchain**: `/api/my/key/blockchain/toggle` - Enable/disable blockchain for a key
  - When enabling: Commits current latest index to blockchain
  - Requires wallet with sufficient balance (0.0001 CHIPS minimum)
  - Stores setting in database
- **Blockchain Status**: `/api/my/key/blockchain/status` - Get blockchain enablement status for all keys
- **Blockchain Explorer**: View all blockchain commits with:
  - Canonical Key IDs (normalized VDXF IDs)
  - User-friendly Key ID labels
  - Actual commit block heights (from identity history)
  - Transaction IDs
  - Bootstrap block height filtering

## 8. Hash Chain Integrity
- **Chain Validation**: Validates hash chain integrity
  - Ensures each entry's hash links to previous entry
  - Special handling for genesis entries (index 0)
  - Detects broken chains
- **Index Monotonicity**: Ensures indices are always incrementing
- **Hash Mismatch Detection**: Detects when hash chain is broken

## 9. Raft Consensus Features
- **Leader Election**: Automatic leader election in 3-node cluster
- **Leader Forwarding**: Non-leader nodes forward requests to leader
- **Fault Tolerance**: Can tolerate 1 node failure (out of 3)
- **Bootstrap Mode**: Node 3 can bootstrap cluster
- **Health Checks**: `/health` endpoint shows cluster health and leader status

## 10. Data Models & Storage
- **Key Index Entry**: Stores key_id, pubkey_hash, index, previous_hash, hash, signature, public_key, record_type, raft_index
- **Record Types**: create, sign, sync, delete
- **Raft Index Tracking**: Each entry tracks its Raft log index for chronological ordering
- **Persistent Storage**: Raft stores data in `raft-data/nodeX/` directories

## 11. Frontend Features
- **Copyable Error Dialog**: Error messages displayed in modal with copyable textarea
- **Real-time Updates**: Recent commits refresh every 5 seconds
- **Incremental Refresh**: Only adds new commits, doesn't reload entire list
- **Search Interface**: Search by key ID, hash, or index
- **Chain Visualization**: View full hash chain for any key
- **Blockchain Explorer**: View all blockchain commits with filtering

## 12. Key ID Generation
- **Format**: `{username}_{number}_{base64_random_32bits}`
- **Example**: `s1_1_drIHzA==`
- **Uniqueness**: Ensures no key ID reuse by incrementing from max existing number
- **Random Suffix**: 32-bit random number (4 bytes) encoded as base64 for uniqueness

## 13. Bootstrap & Configuration
- **Bootstrap Block Height**: Hardcoded to 2737418 (configurable via `LMS_BOOTSTRAP_BLOCK_HEIGHT`)
- **Identity Name**: Configurable Verus identity (default: `sg777z.chips.vrsc@`)
- **Raft Endpoints**: Configurable Raft cluster endpoints
- **Database Paths**: Separate databases for users, wallets, key blockchain settings

## 14. Error Handling
- **Copyable Error Messages**: Errors displayed in modal with selectable text
- **Graceful Degradation**: Explorer works even if HSM server is down (read-only mode)
- **Validation Errors**: Clear error messages for validation failures
- **Blockchain Errors**: Detailed error messages for blockchain operation failures


## 1. User Authentication & Management
- **User Registration**: `/api/auth/register` - Register new users
- **User Login**: `/api/auth/login` - Authenticate users, returns JWT token
- **Get Current User**: `/api/auth/me` - Get authenticated user info
- **JWT Token Authentication**: All authenticated endpoints require valid JWT token

## 2. LMS Key Management (HSM Server)
- **Generate Key**: `/api/my/generate` - Generate new LMS key with parameters
  - Supports username-based key ID format: `{username}_{number}_{random_suffix}`
  - Stores keys in local database
  - Commits create record (index 0) to Raft chain
- **List Keys**: `/api/my/keys` - List all keys for authenticated user
- **Export Key**: `/api/my/export` - Export key (including private key data)
- **Import Key**: `/api/my/import` - Import previously exported key
- **Delete Key**: `/api/my/delete` - Delete a key
  - Commits delete record to Raft chain before deletion
  - Cleans up blockchain settings for deleted key

## 3. Message Signing & Verification
- **Sign Message**: `/api/my/sign` - Sign a message with specified key
  - Commits sign record to Raft chain (index increment)
  - Optionally commits to blockchain if enabled for the key
- **Verify Signature**: `/api/my/verify` - Verify a signature against message and public key

## 4. Raft Chain Operations
- **Commit Index**: `/commit_index` - Commit index update to Raft (used by HSM server)
- **Get All Entries**: `/all_entries?limit=N` - Get all entries ordered by Raft log index (newest first)
- **Get Key Chain**: `/key/{key_id}/chain` - Get full hash chain for a key ID
- **Get Pubkey Hash Chain**: `/pubkey_hash/{pubkey_hash}/index` - Get index by pubkey_hash
- **Get Latest Index**: `/key/{key_id}/index` - Get latest index for a key ID
- **List All Keys**: `/keys` - Get all key IDs in the system

## 5. Explorer Public Interface
- **Recent Commits**: `/api/recent?limit=N` - Get recent commits from Raft (top 10, refreshes every 5 seconds)
- **Search**: `/api/search?q={query}` - Search by key_id, hash, or index
- **Statistics**: `/api/stats` - Get overall statistics (total keys, commits, valid/broken chains)
- **Chain View**: `/api/chain/{key_id}` - Get full chain for a key ID
  - Handles multiple pubkey_hashes for same key_id
  - Shows all historical entries across all pubkey_hashes
- **Blockchain Commits**: `/api/blockchain` - Get all blockchain commits
  - Filters by bootstrap block height
  - Shows actual commit block heights from identity history
  - Displays canonical key IDs and user-friendly labels

## 6. Wallet Management (CHIPS)
- **List Wallets**: `/api/my/wallet/list` - List all wallets for user
- **Create Wallet**: `/api/my/wallet/create` - Create new CHIPS wallet address
- **Get Balance**: `/api/my/wallet/balance?address={addr}` - Get balance for specific address
- **Get Total Balance**: `/api/my/wallet/total-balance` - Get total balance across all user's wallets

## 7. Blockchain Integration (Verus/CHIPS)
- **Toggle Blockchain**: `/api/my/key/blockchain/toggle` - Enable/disable blockchain for a key
  - When enabling: Commits current latest index to blockchain
  - Requires wallet with sufficient balance (0.0001 CHIPS minimum)
  - Stores setting in database
- **Blockchain Status**: `/api/my/key/blockchain/status` - Get blockchain enablement status for all keys
- **Blockchain Explorer**: View all blockchain commits with:
  - Canonical Key IDs (normalized VDXF IDs)
  - User-friendly Key ID labels
  - Actual commit block heights (from identity history)
  - Transaction IDs
  - Bootstrap block height filtering

## 8. Hash Chain Integrity
- **Chain Validation**: Validates hash chain integrity
  - Ensures each entry's hash links to previous entry
  - Special handling for genesis entries (index 0)
  - Detects broken chains
- **Index Monotonicity**: Ensures indices are always incrementing
- **Hash Mismatch Detection**: Detects when hash chain is broken

## 9. Raft Consensus Features
- **Leader Election**: Automatic leader election in 3-node cluster
- **Leader Forwarding**: Non-leader nodes forward requests to leader
- **Fault Tolerance**: Can tolerate 1 node failure (out of 3)
- **Bootstrap Mode**: Node 3 can bootstrap cluster
- **Health Checks**: `/health` endpoint shows cluster health and leader status

## 10. Data Models & Storage
- **Key Index Entry**: Stores key_id, pubkey_hash, index, previous_hash, hash, signature, public_key, record_type, raft_index
- **Record Types**: create, sign, sync, delete
- **Raft Index Tracking**: Each entry tracks its Raft log index for chronological ordering
- **Persistent Storage**: Raft stores data in `raft-data/nodeX/` directories

## 11. Frontend Features
- **Copyable Error Dialog**: Error messages displayed in modal with copyable textarea
- **Real-time Updates**: Recent commits refresh every 5 seconds
- **Incremental Refresh**: Only adds new commits, doesn't reload entire list
- **Search Interface**: Search by key ID, hash, or index
- **Chain Visualization**: View full hash chain for any key
- **Blockchain Explorer**: View all blockchain commits with filtering

## 12. Key ID Generation
- **Format**: `{username}_{number}_{base64_random_32bits}`
- **Example**: `s1_1_drIHzA==`
- **Uniqueness**: Ensures no key ID reuse by incrementing from max existing number
- **Random Suffix**: 32-bit random number (4 bytes) encoded as base64 for uniqueness

## 13. Bootstrap & Configuration
- **Bootstrap Block Height**: Hardcoded to 2737418 (configurable via `LMS_BOOTSTRAP_BLOCK_HEIGHT`)
- **Identity Name**: Configurable Verus identity (default: `sg777z.chips.vrsc@`)
- **Raft Endpoints**: Configurable Raft cluster endpoints
- **Database Paths**: Separate databases for users, wallets, key blockchain settings

## 14. Error Handling
- **Copyable Error Messages**: Errors displayed in modal with selectable text
- **Graceful Degradation**: Explorer works even if HSM server is down (read-only mode)
- **Validation Errors**: Clear error messages for validation failures
- **Blockchain Errors**: Detailed error messages for blockchain operation failures


## 1. User Authentication & Management
- **User Registration**: `/api/auth/register` - Register new users
- **User Login**: `/api/auth/login` - Authenticate users, returns JWT token
- **Get Current User**: `/api/auth/me` - Get authenticated user info
- **JWT Token Authentication**: All authenticated endpoints require valid JWT token

## 2. LMS Key Management (HSM Server)
- **Generate Key**: `/api/my/generate` - Generate new LMS key with parameters
  - Supports username-based key ID format: `{username}_{number}_{random_suffix}`
  - Stores keys in local database
  - Commits create record (index 0) to Raft chain
- **List Keys**: `/api/my/keys` - List all keys for authenticated user
- **Export Key**: `/api/my/export` - Export key (including private key data)
- **Import Key**: `/api/my/import` - Import previously exported key
- **Delete Key**: `/api/my/delete` - Delete a key
  - Commits delete record to Raft chain before deletion
  - Cleans up blockchain settings for deleted key

## 3. Message Signing & Verification
- **Sign Message**: `/api/my/sign` - Sign a message with specified key
  - Commits sign record to Raft chain (index increment)
  - Optionally commits to blockchain if enabled for the key
- **Verify Signature**: `/api/my/verify` - Verify a signature against message and public key

## 4. Raft Chain Operations
- **Commit Index**: `/commit_index` - Commit index update to Raft (used by HSM server)
- **Get All Entries**: `/all_entries?limit=N` - Get all entries ordered by Raft log index (newest first)
- **Get Key Chain**: `/key/{key_id}/chain` - Get full hash chain for a key ID
- **Get Pubkey Hash Chain**: `/pubkey_hash/{pubkey_hash}/index` - Get index by pubkey_hash
- **Get Latest Index**: `/key/{key_id}/index` - Get latest index for a key ID
- **List All Keys**: `/keys` - Get all key IDs in the system

## 5. Explorer Public Interface
- **Recent Commits**: `/api/recent?limit=N` - Get recent commits from Raft (top 10, refreshes every 5 seconds)
- **Search**: `/api/search?q={query}` - Search by key_id, hash, or index
- **Statistics**: `/api/stats` - Get overall statistics (total keys, commits, valid/broken chains)
- **Chain View**: `/api/chain/{key_id}` - Get full chain for a key ID
  - Handles multiple pubkey_hashes for same key_id
  - Shows all historical entries across all pubkey_hashes
- **Blockchain Commits**: `/api/blockchain` - Get all blockchain commits
  - Filters by bootstrap block height
  - Shows actual commit block heights from identity history
  - Displays canonical key IDs and user-friendly labels

## 6. Wallet Management (CHIPS)
- **List Wallets**: `/api/my/wallet/list` - List all wallets for user
- **Create Wallet**: `/api/my/wallet/create` - Create new CHIPS wallet address
- **Get Balance**: `/api/my/wallet/balance?address={addr}` - Get balance for specific address
- **Get Total Balance**: `/api/my/wallet/total-balance` - Get total balance across all user's wallets

## 7. Blockchain Integration (Verus/CHIPS)
- **Toggle Blockchain**: `/api/my/key/blockchain/toggle` - Enable/disable blockchain for a key
  - When enabling: Commits current latest index to blockchain
  - Requires wallet with sufficient balance (0.0001 CHIPS minimum)
  - Stores setting in database
- **Blockchain Status**: `/api/my/key/blockchain/status` - Get blockchain enablement status for all keys
- **Blockchain Explorer**: View all blockchain commits with:
  - Canonical Key IDs (normalized VDXF IDs)
  - User-friendly Key ID labels
  - Actual commit block heights (from identity history)
  - Transaction IDs
  - Bootstrap block height filtering

## 8. Hash Chain Integrity
- **Chain Validation**: Validates hash chain integrity
  - Ensures each entry's hash links to previous entry
  - Special handling for genesis entries (index 0)
  - Detects broken chains
- **Index Monotonicity**: Ensures indices are always incrementing
- **Hash Mismatch Detection**: Detects when hash chain is broken

## 9. Raft Consensus Features
- **Leader Election**: Automatic leader election in 3-node cluster
- **Leader Forwarding**: Non-leader nodes forward requests to leader
- **Fault Tolerance**: Can tolerate 1 node failure (out of 3)
- **Bootstrap Mode**: Node 3 can bootstrap cluster
- **Health Checks**: `/health` endpoint shows cluster health and leader status

## 10. Data Models & Storage
- **Key Index Entry**: Stores key_id, pubkey_hash, index, previous_hash, hash, signature, public_key, record_type, raft_index
- **Record Types**: create, sign, sync, delete
- **Raft Index Tracking**: Each entry tracks its Raft log index for chronological ordering
- **Persistent Storage**: Raft stores data in `raft-data/nodeX/` directories

## 11. Frontend Features
- **Copyable Error Dialog**: Error messages displayed in modal with copyable textarea
- **Real-time Updates**: Recent commits refresh every 5 seconds
- **Incremental Refresh**: Only adds new commits, doesn't reload entire list
- **Search Interface**: Search by key ID, hash, or index
- **Chain Visualization**: View full hash chain for any key
- **Blockchain Explorer**: View all blockchain commits with filtering

## 12. Key ID Generation
- **Format**: `{username}_{number}_{base64_random_32bits}`
- **Example**: `s1_1_drIHzA==`
- **Uniqueness**: Ensures no key ID reuse by incrementing from max existing number
- **Random Suffix**: 32-bit random number (4 bytes) encoded as base64 for uniqueness

## 13. Bootstrap & Configuration
- **Bootstrap Block Height**: Hardcoded to 2737418 (configurable via `LMS_BOOTSTRAP_BLOCK_HEIGHT`)
- **Identity Name**: Configurable Verus identity (default: `sg777z.chips.vrsc@`)
- **Raft Endpoints**: Configurable Raft cluster endpoints
- **Database Paths**: Separate databases for users, wallets, key blockchain settings

## 14. Error Handling
- **Copyable Error Messages**: Errors displayed in modal with selectable text
- **Graceful Degradation**: Explorer works even if HSM server is down (read-only mode)
- **Validation Errors**: Clear error messages for validation failures
- **Blockchain Errors**: Detailed error messages for blockchain operation failures

