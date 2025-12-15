# Quick Start Guide

## Build All Components

**Recommended: Use the build script**
```bash
cd /root/lms
./build.sh
```

This builds all components:
- `lms-service` - Main Raft service
- `lms-explorer` - Web explorer interface
- `hsm-server` - HSM server
- `hsm-client` - HSM client tool

**Or build individually:**
```bash
# Build main service
go build -o lms-service ./main.go

# Build explorer
go build -o lms-explorer ./cmd/explorer

# Build HSM server
go build -o hsm-server ./cmd/hsm-server

# Build HSM client
go build -o hsm-client ./cmd/hsm-client
```

## Run on 3 Nodes

### Node 1 (Bootstrap - Run First!)
```bash
./lms-service -id node1 -addr 159.69.23.29:7000 -bootstrap
```

### Node 2
```bash
./lms-service -id node2 -addr 159.69.23.30:7000
```

### Node 3
```bash
./lms-service -id node3 -addr 159.69.23.31:7000
```

## Test Endpoints

### Health Check
```bash
curl http://159.69.23.29:8080/health
```

### Get Leader
```bash
curl http://159.69.23.29:8080/leader
```

### Get Latest Attestation
```bash
curl http://159.69.23.29:8080/latest-head
```

### Submit Attestation
```bash
curl -X POST http://159.69.23.29:8080/propose \
  -H "Content-Type: application/json" \
  -d '{
    "attestation": {
      "attestation_response": {
        "policy": {"value": "LMS_ATTEST_POLICY", "algorithm": "PS256"},
        "data": {
          "value": "eyJwcmV2aW91c19oYXNoIjoibG1zX2dlbmVzaXNfaGFzaF92ZXJpZmlhYmxlX3N0YXRlX2NoYWlucyIsImxtc19pbmRleCI6MCwibWVzc2FnZV9zaWduZWQiOiJtZXNzYWdlX2hhc2hfMCIsInNlcXVlbmNlX251bWJlciI6MCwidGltZXN0YW1wIjoiMjAyNS0wMS0xNVQxMjowMDowMFoiLCJtZXRhZGF0YSI6ImdlbmVzaXMifQ==",
          "encoding": "base64"
        },
        "signature": {"value": "dGVzdA==", "encoding": "base64"},
        "certificate": {"value": "dGVzdA==", "encoding": "pem"}
      }
    },
    "hsm_identifier": "hsm1"
  }'
```

## Command-Line Flags

- `-id`: Node ID (node1, node2, node3)
- `-addr`: Raft address (IP:port)
- `-api-port`: HTTP API port (default: 8080)
- `-raft-port`: Raft transport port (default: 7000)
- `-raft-dir`: Data directory (default: ./raft-data)
- `-bootstrap`: Bootstrap cluster (only on first node)
- `-genesis-hash`: Genesis hash (default: lms_genesis_hash_verifiable_state_chains)

## Start Explorer

After the Raft cluster is running, start the web explorer:

```bash
# With logging to file (recommended)
./lms-explorer -port 8081 -log-file explorer.log

# Or with default settings
./lms-explorer -port 8081
```

Access at: `http://localhost:8081`

See [Explorer README](../explorer/README.md) for detailed usage.

## Start HSM Server

The HSM server handles signing operations:

```bash
./hsm-server -port 9090
```

The explorer connects to the HSM server at the configured endpoint (default: `http://159.69.23.31:9090`).

## Default Ports

- **7000**: Raft internal communication
- **8080**: HTTP API for HSM clients
- **8081**: Explorer web interface
- **9090**: HSM server

