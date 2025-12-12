# Verifiable State Chains

A fault-tolerant Raft-based architecture for managing stateful Hash-Based Signature (LMS) state in distributed HSM clusters.

## Overview

This implementation provides a replicated log service built on the Raft consensus protocol to manage LMS index state, preventing catastrophic index reuse in high-availability Hardware Security Module (HSM) deployments.

## Architecture

- **Hash-Chain Log Format**: Lightweight verifiable attestation chain
- **Raft Consensus**: Crash-fault tolerant replicated log
- **HSM Client Protocol**: Complete workflow for attestation commitment

## Project Structure

```
/root/lms/
├── models/          # Data models (attestations, log entries, API types)
├── service/         # Raft service layer (API, leader forwarding)
├── fsm/            # Hash-chain FSM implementation
├── client/         # HSM client protocol
├── validation/     # Validation and security checks
├── simulator/      # HSM simulator for testing
└── tests/          # Integration tests
```

## Current Status

✅ **Module 1**: Project Structure & Data Models (COMPLETE)
- All data structures defined
- Serialization/deserialization working
- Unit tests passing

## Testing

### Module 1 Tests

```bash
cd /root/lms
go test ./models -v
```

### Node Configuration

The system is designed for 3-node Raft cluster:
- Node 1: 159.69.23.29:7000
- Node 2: 159.69.23.30:7000
- Node 3: 159.69.23.31:7000

## Dependencies

- Go 1.21+
- HashiCorp Raft library
- BoltDB for persistence

## References

Based on: "Verifiable State Chains: A Fault-Tolerant Raft-Based Architecture for Stateful HBS Management"

