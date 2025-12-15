# Verifiable State Chains

A fault-tolerant Raft-based architecture for managing stateful Hash-Based Signature (LMS) state in distributed HSM clusters with Verus blockchain integration.

## Overview

This implementation provides a replicated log service built on the Raft consensus protocol to manage LMS index state, preventing catastrophic index reuse in high-availability Hardware Security Module (HSM) deployments. The system includes:

- **Raft-based State Management**: Crash-fault tolerant replicated log
- **HSM Integration**: Complete workflow for attestation commitment
- **Web Explorer**: Full-featured web interface for browsing and managing hash chains
- **Blockchain Integration**: Optional Verus/CHIPS blockchain commits for additional redundancy
- **Wallet Management**: CHIPS wallet integration for blockchain transaction funding

## Quick Start

### Build All Components

```bash
cd /root/lms
./build.sh
```

This builds all components:
- `lms-service` - Main Raft service
- `lms-explorer` - Web explorer interface
- `hsm-server` - HSM server
- `hsm-client` - HSM client tool

### Start the Raft Cluster

Start 3 nodes (minimum for fault tolerance):

```bash
# Node 1
./lms-service -id node1 -port 7000 -http-port 8080

# Node 2
./lms-service -id node2 -port 7000 -http-port 8080

# Node 3
./lms-service -id node3 -port 7000 -http-port 8080
```

### Start the Explorer

```bash
./lms-explorer -port 8081 -log-file explorer.log
```

Access at: `http://localhost:8081`

## Project Structure

```
/root/lms/
├── main.go              # Main Raft service entry point
├── build.sh             # Build script for all components
├── models/              # Data models (attestations, log entries, API types)
├── service/             # Raft service layer (API, leader forwarding)
├── fsm/                 # Hash-chain FSM implementation
├── client/              # HSM client protocol
├── hsm_server/          # HSM server implementation
├── hsm_client/          # HSM client library
├── explorer/            # Web explorer (frontend + backend)
│   ├── server.go        # HTTP server and routing
│   ├── wallet.go        # CHIPS wallet management
│   ├── blockchain.go    # Verus blockchain integration
│   └── static/          # Frontend assets
├── blockchain/          # Verus/CHIPS RPC client
├── validation/          # Validation and security checks
├── simulator/           # HSM simulator for testing
├── cmd/                 # Command-line tools
│   ├── explorer/       # Explorer binary
│   ├── hsm-server/     # HSM server binary
│   └── hsm-client/     # HSM client binary
└── tests/              # Integration tests
```

## Components

### 1. LMS Service (Raft Cluster)

The core Raft-based service that manages LMS index state.

**Build:**
```bash
go build -o lms-service ./main.go
```

**Run:**
```bash
./lms-service -id node1 -port 7000 -http-port 8080
```

### 2. Explorer

Web-based interface for browsing hash chains, managing keys, and viewing blockchain commits.

**Features:**
- Browse recent commits
- Search by key ID or hash
- View full hash chains
- User authentication
- Key management (generate, import, export, delete)
- CHIPS wallet management
- Per-key blockchain toggle
- View blockchain commits

**Build:**
```bash
go build -o lms-explorer ./cmd/explorer
```

**Run:**
```bash
# With logging to file
./lms-explorer -port 8081 -log-file explorer.log

# With default settings
./lms-explorer -port 8081
```

**Options:**
- `-port` - Web server port (default: 8081)
- `-raft-endpoints` - Comma-separated Raft endpoints
- `-hsm-endpoint` - HSM server endpoint
- `-log-file` - Optional log file path

### 3. HSM Server

Handles signing operations and manages LMS keys.

**Build:**
```bash
go build -o hsm-server ./cmd/hsm-server
```

**Run:**
```bash
./hsm-server -port 9090
```

### 4. HSM Client

Command-line tool for interacting with the HSM server.

**Build:**
```bash
go build -o hsm-client ./cmd/hsm-client
```

## Features

### Hash Chain Management
- ✅ Raft-based replication
- ✅ Chain integrity verification
- ✅ Genesis entry tracking
- ✅ Index management

### Web Explorer
- ✅ Real-time commit browsing
- ✅ Search by key ID or hash
- ✅ Chain visualization
- ✅ Statistics dashboard
- ✅ User authentication
- ✅ Key management UI

### Wallet & Blockchain
- ✅ CHIPS wallet creation and management
- ✅ Balance checking
- ✅ Per-key blockchain toggle
- ✅ Automatic blockchain commits
- ✅ Verus identity integration
- ✅ Transaction funding from user wallets

### Security
- ✅ JWT authentication
- ✅ User isolation
- ✅ Balance checks before operations
- ✅ Explicit funding address support

## Node Configuration

The system is designed for a 3-node Raft cluster:
- Node 1: 159.69.23.29:7000 (Raft), :8080 (HTTP)
- Node 2: 159.69.23.30:7000 (Raft), :8080 (HTTP)
- Node 3: 159.69.23.31:7000 (Raft), :8080 (HTTP)

## Dependencies

- Go 1.21+
- HashiCorp Raft library
- BoltDB for persistence
- Verus/CHIPS node (for blockchain features)

## Testing

### Run All Tests
```bash
go test ./... -v
```

### Test Specific Module
```bash
go test ./models -v
go test ./fsm -v
go test ./service -v
```

## Documentation

See `docs/` directory for detailed documentation:
- [Quick Start Guide](docs/QUICK_START.md)
- [Testing Guide](docs/TESTING_GUIDE.md)
- [Explorer README](explorer/README.md)
- [Architecture Documentation](docs/ARCHITECTURE.md)

## References

Based on: "Verifiable State Chains: A Fault-Tolerant Raft-Based Architecture for Stateful HBS Management"
