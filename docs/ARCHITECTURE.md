# LMS System Architecture

## Clear Separation of Concerns

### 1. **HSM Server** (`hsm_server/`)
**Role**: The authoritative source for all key operations
- Manages LMS keys (generate, store, delete)
- Signs messages with LMS keys
- **Commits indices to Raft cluster** (primary)
- **Commits indices to Verus blockchain** (for testing/fallback)
- Has access to CHIPS funds for blockchain transactions
- Stores keys in local database (`keys.db`)

**Location**: Runs as a separate service (typically on port 9090)

**Key Files**:
- `hsm_server/hsm_server.go` - Main server implementation
- `hsm_server/sign.go` - Signing logic and Raft/blockchain commits
- `hsm_server/keys.go` - Key management

### 2. **Explorer** (`explorer/`)
**Role**: Web UI frontend that PROXIES requests to HSM server
- Provides web interface for users
- Handles user authentication (registration, login)
- **Forwards all key/signing requests to HSM server** (does NOT handle locally)
- Queries Raft cluster for chain data (read-only)

**Location**: Runs as a separate service (typically on port 8081)

**Key Files**:
- `explorer/server.go` - Main explorer server
- `explorer/hsm_proxy.go` - Proxy handlers that forward to HSM server
- `explorer/auth.go` - User authentication

### 3. **Raft Service** (`service/`)
**Role**: Distributed consensus for index commits
- Receives index commits from HSM server
- Stores hash chain data
- Provides API for querying chains (read-only)

**Location**: Runs as a cluster (typically on ports 8080)

### 4. **Client Protocol** (`client/`)
**Role**: Library for clients to interact with the system
- Used for programmatic access
- Does NOT commit to blockchain (HSM server does that)
- Commits attestations to Raft service

## Request Flow

### Signing a Message (via Explorer):
```
User → Explorer UI → Explorer Server (/api/my/sign)
  → Proxies to → HSM Server (/sign)
    → Signs message
    → Commits index to Raft
    → Commits index to Blockchain (if enabled)
    → Returns signature to Explorer
      → Returns to User
```

### Generating a Key (via Explorer):
```
User → Explorer UI → Explorer Server (/api/my/generate)
  → Proxies to → HSM Server (/generate_key)
    → Generates LMS key
    → Stores in HSM server database
    → Returns key info to Explorer
      → Returns to User
```

## Important Points

1. **HSM Server is REQUIRED**: Explorer cannot function without HSM server running
2. **Blockchain commits ONLY in HSM Server**: Only HSM server has CHIPS funds and commits to blockchain
3. **Explorer is just a proxy**: All actual key operations happen in HSM server
4. **No local storage in Explorer**: Explorer has no key database, just forwards requests

## Configuration

### HSM Server Startup
```go
blockchainConfig := &hsm_server.BlockchainConfig{
    Enabled:      true,
    RPCURL:       "http://127.0.0.1:22778",
    RPCUser:      "user1172159772",
    RPCPassword:  "pass...",
    IdentityName: "sg777z.chips.vrsc@",
}

server, err := hsm_server.NewHSMServer(9090, raftEndpoints, blockchainConfig)
```

### Explorer Startup
```go
server, err := explorer.NewExplorerServer(8081, raftEndpoints, "http://localhost:9090")
// Explorer needs HSM server endpoint to proxy requests
```

