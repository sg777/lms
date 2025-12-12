# Quick Start Guide

## Build

```bash
cd /root/lms
go build -o lms-service .
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

## Default Ports

- **7000**: Raft internal communication
- **8080**: HTTP API for HSM clients

