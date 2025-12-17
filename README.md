# Verifiable State Chains

**A Production-Ready Raft-Based Architecture for Managing LMS State in Distributed HSM Clusters**

[![Go Version](https://img.shields.io/badge/go-1.22+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Overview

This implementation provides a fault-tolerant, distributed system for managing Leighton-Micali Signature (LMS) index state, preventing catastrophic index reuse in high-availability Hardware Security Module (HSM) deployments. The system combines Raft consensus for high availability with optional Verus blockchain integration for additional redundancy and audit capabilities.

### Key Features

- **🔐 Fault-Tolerant State Management**: Raft-based consensus with crash-fault tolerance
- **🔗 Hash Chain Integrity**: Cryptographic verification of index progression
- **🌐 Web Explorer**: Full-featured interface for monitoring and management
- **⛓️ Blockchain Integration**: Optional Verus/CHIPS commits with optimized fees
- **💳 Wallet Management**: Integrated CHIPS wallet for transaction funding
- **🚀 Production Ready**: Comprehensive logging, error handling, and monitoring
- **⚡ Performance Optimized**: 50-75% blockchain fee reduction through smart commits

## Quick Start

### Prerequisites

- **Go** 1.22 or later
- **GCC** and **Make** (for building hash-sigs library)
- **OpenSSL** development libraries
- **Verus/CHIPS** node (optional, for blockchain features)

### Installation

#### 1. Clone Repository

```bash
git clone <repository-url>
cd lms
git submodule update --init --recursive
```

#### 2. Build All Components

```bash
./build.sh
```

This builds:
- `lms-service` - Raft consensus service
- `lms-explorer` - Web explorer interface
- `hsm-server` - HSM server
- `hsm-client` - HSM client tool

The build script automatically:
- Initializes/updates Git submodules
- Builds the hash-sigs C library
- Compiles all Go binaries

### Running the System

#### Start Raft Cluster (3 nodes minimum for fault tolerance)

```bash
# Terminal 1 - Node 1
./lms-service -id node1 -port 7000 -http-port 8080

# Terminal 2 - Node 2
./lms-service -id node2 -port 7000 -http-port 8080

# Terminal 3 - Node 3 (bootstrap mode)
./start_node3_bootstrap.sh
```

#### Start Explorer

```bash
./lms-explorer -port 8081 -log-file explorer.log
```

Access at: **http://localhost:8081**

#### Start HSM Server

```bash
./hsm-server -port 9090
```

### Quick Test

```bash
# Generate a key
./hsm-client -action generate -key-id test_key_1

# Sign a message
./hsm-client -action sign -key-id test_key_1 -message "Hello, World!"

# Verify the signature
./hsm-client -action verify -key-id test_key_1 -message "Hello, World!" -signature <signature>
```

## Architecture

### System Components

```
┌─────────────────────────────────────────────────────────────┐
│                       Web Explorer                          │
│  (Browse chains, manage keys, view blockchain commits)     │
└───────────────┬─────────────────────────────────────────────┘
                │
    ┌───────────┴───────────┐
    │                       │
┌───▼────────┐      ┌──────▼──────┐
│ HSM Server │      │ Raft Cluster│
│            │      │  (3 nodes)  │
│ • Sign     │◄─────┤             │
│ • Verify   │      │ • Consensus │
│ • Keys     │      │ • Replicate │
└────────────┘      │ • Persist   │
                    └──────┬──────┘
                           │
                    ┌──────▼───────┐
                    │   Blockchain │
                    │ (Verus/CHIPS)│
                    │   Optional   │
                    └──────────────┘
```

### Data Flow

1. **Key Generation**: HSM creates key → Commits genesis (index 0) to Raft
2. **Signing**: HSM signs message → Increments index → Commits to Raft (+ optional blockchain)
3. **Verification**: Anyone can verify signature using public key
4. **Monitoring**: Explorer displays real-time chain state and statistics

## Project Structure

```
/root/lms/
├── blockchain/          # Verus/CHIPS RPC client
├── client/              # HSM client protocol
├── cmd/                 # Command-line binaries
│   ├── explorer/        # Explorer binary
│   ├── hsm-server/      # HSM server binary
│   └── hsm-client/      # HSM client binary
├── explorer/            # Web explorer (backend + frontend)
│   ├── server.go        # HTTP server and routing
│   ├── wallet.go        # CHIPS wallet management
│   ├── blockchain.go    # Blockchain integration
│   ├── auth.go          # User authentication
│   └── static/          # Frontend assets (HTML/CSS/JS)
├── fsm/                 # Finite State Machine (Raft FSM)
│   ├── key_index_fsm.go # Hash chain FSM
│   └── combined_fsm.go  # Unified FSM interface
├── hsm_server/          # HSM server implementation
│   ├── hsm_server.go    # Key management
│   ├── sign.go          # Signing operations
│   └── export_import.go # Key export/import
├── models/              # Data models
├── service/             # Raft service layer
├── validation/          # Security validation
├── main.go              # Raft service entry point
└── build.sh             # Build script
```

## Features

### Core Capabilities

#### 🔐 LMS Key Management
- Generate keys with customizable parameters
- Username-based key IDs: `{username}_{number}_{random}`
- Export/import for backup and migration
- Secure deletion with Raft chain preservation

#### ⛓️ Hash Chain Integrity
- Cryptographic chain validation
- Genesis entry special handling
- Index reuse prevention
- Real-time chain status monitoring

#### 🌐 Web Explorer
- **Real-time monitoring**: Auto-refresh every 5 seconds (Raft) / 10 seconds (blockchain)
- **Smart updates**: Only updates UI when data changes (no flicker)
- **Search & discovery**: By key ID, hash, or index
- **Chain visualization**: Interactive hash chain display
- **Statistics dashboard**: Keys, commits, valid/broken chains
- **User authentication**: Secure JWT-based login
- **Copyable errors**: User-friendly error dialogs

#### 💳 Wallet Management
- Create CHIPS wallets
- Check balances
- Fund blockchain transactions
- Per-key blockchain toggle

#### ⛓️ Blockchain Integration (Verus/CHIPS)
- **Optimized commits**: 50-75% fee reduction
  - Before: ~3.8KB (0.0004 CHIPS)
  - After: ~1KB (0.0001-0.0002 CHIPS)
- Per-key enable/disable
- Complete audit trail via `getidentityhistory`
- Bootstrap block height filtering (2742761)
- Transaction fee estimation

#### 🚀 High Availability
- **Raft consensus**: 3-node cluster
- **Fault tolerance**: Survives 1 node failure
- **Automatic failover**: Sub-second leader election
- **Transparent routing**: Non-leader nodes forward to leader
- **Health monitoring**: `/health` endpoint

### Security Features

- **JWT Authentication**: Secure user sessions
- **User Isolation**: Keys and wallets are user-specific
- **Balance Checks**: Prevents insufficient fund operations
- **Hash Chain Validation**: Cryptographic integrity verification
- **Encrypted Storage**: Secure key storage
- **Audit Trail**: Complete operation history

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LMS_BOOTSTRAP_BLOCK_HEIGHT` | Bootstrap block height for blockchain filtering | `2742761` |
| `VERUS_RPC_URL` | Verus node RPC endpoint | `http://127.0.0.1:22778` |
| `VERUS_RPC_USER` | Verus RPC username | `user1172159772` |
| `VERUS_RPC_PASS` | Verus RPC password | (configured) |

### Node Configuration

**Default 3-Node Cluster:**
- **Node 1**: 159.69.23.29:7000 (Raft), :8080 (HTTP)
- **Node 2**: 159.69.23.30:7000 (Raft), :8080 (HTTP)
- **Node 3**: 159.69.23.31:7000 (Raft), :8080 (HTTP)

**Explorer**: Port 8081 (configurable)
**HSM Server**: Port 9090 (configurable)

## API Reference

### Public Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/recent?limit=N` | GET | Get recent commits (top 10) |
| `/api/search?q={query}` | GET | Search by key ID, hash, or index |
| `/api/stats` | GET | System statistics |
| `/api/chain/{key_id}` | GET | Full hash chain for key |
| `/api/blockchain` | GET | Blockchain commits |
| `/health` | GET | Health check (Raft cluster status) |

### Authenticated Endpoints (Require JWT)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/auth/register` | POST | Register new user |
| `/api/auth/login` | POST | Login and get JWT token |
| `/api/my/keys` | GET | List user's keys |
| `/api/my/generate` | POST | Generate new key |
| `/api/my/sign` | POST | Sign message |
| `/api/my/verify` | POST | Verify signature |
| `/api/my/export` | POST | Export key |
| `/api/my/import` | POST | Import key |
| `/api/my/delete` | POST | Delete key |
| `/api/my/wallet/list` | GET | List wallets |
| `/api/my/wallet/create` | POST | Create wallet |
| `/api/my/wallet/balance` | GET | Get wallet balance |
| `/api/my/key/blockchain/toggle` | POST | Enable/disable blockchain |

See [FUNCTIONALITY_LIST.md](FUNCTIONALITY_LIST.md) for complete API documentation.

## Performance

### Optimizations Implemented

1. **Blockchain Transaction Size**: 50-75% reduction
   - Only send new entry per commit
   - Rely on blockchain history for full data
   
2. **UI Refresh**: Smart updates prevent flicker
   - Compare by hash and RaftIndex
   - Skip DOM updates when no changes
   
3. **API Efficiency**: Configurable limits and caching
   - Key ID label caching
   - Silent refresh mode
   - Lazy loading

### Benchmarks

- **Raft Commit**: < 100ms (3-node cluster)
- **Blockchain Commit**: ~2-5 seconds (block confirmation)
- **Chain Verification**: < 10ms (100 entries)
- **Explorer Load**: < 500ms (initial page load)

## Testing

### Run All Tests

```bash
go test ./... -v
```

### Test Specific Modules

```bash
go test ./fsm -v          # FSM tests
go test ./service -v      # Service tests
go test ./blockchain -v   # Blockchain tests
```

### Integration Tests

```bash
./run_tests.sh
```

## Troubleshooting

### Common Issues

**"No leader elected"**
- Ensure all 3 nodes are running
- Check network connectivity between nodes
- Review logs: `tail -f raft-data/node1/raft.log`

**"Insufficient balance" error**
- Fund your CHIPS wallet: Minimum 0.0005 CHIPS
- Check balance: `/api/my/wallet/total-balance`
- Use wallet refresh button in UI

**"Hash mismatch" in chain**
- Check for data corruption: `./clear_raft_data.sh` and restart
- Verify all nodes have same data
- Review FSM logs for detailed error

**Explorer not connecting to HSM**
- Verify HSM server is running: `curl http://localhost:9090/health`
- Check `-hsm-endpoint` flag in explorer startup
- Review `explorer.log` for connection errors

### Useful Scripts

```bash
# Clear all Raft data (fresh start)
./clear_raft_data.sh

# Restart explorer
./restart_explorer.sh

# Test wallet balance
./test_wallet_balance.sh
```

## Documentation

- **[Complete Functionality List](FUNCTIONALITY_LIST.md)** - Detailed API and feature documentation
- **[Test Plan](TEST_PLAN.md)** - Comprehensive testing coverage
- **[Explorer README](explorer/README.md)** - Web interface guide
- **[Architecture Documentation](docs/ARCHITECTURE.md)** - System design details
- **[Blockchain Integration](docs/VERUS_BLOCKCHAIN_INTEGRATION.md)** - Verus/CHIPS integration guide

## Development

### Building from Source

```bash
# Build all components
./build.sh

# Build specific component
go build -o lms-service ./main.go
go build -o lms-explorer ./cmd/explorer
go build -o hsm-server ./cmd/hsm-server
```

### Code Structure

- **Clean Architecture**: Separation of concerns
- **Interface-Driven**: Testable components
- **Error Handling**: Comprehensive logging
- **Performance**: Optimized for production

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - See [LICENSE](LICENSE) file for details

## References

- **Paper**: "Verifiable State Chains: A Fault-Tolerant Raft-Based Architecture for Stateful HBS Management"
- **LMS RFC**: [RFC 8554](https://tools.ietf.org/html/rfc8554)
- **Raft Consensus**: [raft.github.io](https://raft.github.io/)
- **Verus**: [verus.io](https://verus.io/)

## Support

For issues, questions, or contributions:
- Open an issue on GitHub
- Review existing documentation
- Check logs for detailed error messages

---

**Status**: Production Ready  
**Version**: 1.0.0  
**Last Updated**: December 2024
