# Verifiable State Chains - Implementation Plan

## Overview
Implementing a fault-tolerant Raft-based architecture for LMS state management using hash-chain log format (no blockchain fallback).

## Module Breakdown

### Module 1: Project Structure & Data Models ✅
**Goal**: Define all data structures and project organization
**Test**: Unit tests for data structure serialization/deserialization
**Files**:
- `models/attestation.go` - Attestation structures
- `models/log_entry.go` - Log entry formats
- `models/request_response.go` - API request/response types

### Module 2: Enhanced Raft Service Layer
**Goal**: Add leader forwarding, API layer, and secure transport
**Test**: Multi-node cluster with leader forwarding, API endpoints respond correctly
**Files**:
- `service/api.go` - HTTP/gRPC API handlers
- `service/leader_forwarding.go` - Leader detection and forwarding
- `service/config.go` - Service configuration

### Module 3: Hash-Chain FSM Implementation
**Goal**: Replace simple string FSM with hash-chain attestation storage
**Test**: FSM correctly stores/retrieves attestations, maintains hash chain integrity
**Files**:
- `fsm/hashchain_fsm.go` - Hash-chain FSM implementation
- `fsm/storage.go` - Persistent storage for attestations

### Module 4: HSM Client Protocol
**Goal**: Implement the complete HSM protocol workflow
**Test**: End-to-end test: HSM client can commit attestation and retrieve state
**Files**:
- `client/hsm_client.go` - HSM client implementation
- `client/protocol.go` - Protocol state machine

### Module 5: Validation & Security Layer
**Goal**: Add strict validation (hash verification, index monotonicity, signatures)
**Test**: Invalid attestations are rejected, valid ones accepted
**Files**:
- `validation/validator.go` - Validation logic
- `validation/crypto.go` - Cryptographic verification

### Module 6: HSM Simulator & Integration Tests
**Goal**: Create HSM simulator and end-to-end integration tests
**Test**: Full workflow with multiple HSMs, failover scenarios
**Files**:
- `simulator/hsm_sim.go` - HSM simulator
- `tests/integration_test.go` - Integration tests

## Testing Strategy

Each module will have:
1. Unit tests for individual components
2. Integration tests with the Raft cluster
3. Manual testing scripts for verification

## Dependencies
- HashiCorp Raft library (already in use)
- Standard Go crypto libraries
- JSON encoding for API
- BoltDB for persistence (already in use)

