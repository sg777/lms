# LMS Integration Plan

## Overview
Integrate actual LMS (Leighton-Micali Signature) key generation and signing into the HSM server.

## Library Choice: Cisco hash-sigs

**Repository**: https://github.com/cisco/hash-sigs
- Full RFC 8554 compliant implementation
- Written in Go (matches our codebase)
- Well-maintained by Cisco
- Supports LMS and HSS (Hierarchical Signature System)

## Implementation Steps

### 1. Add Persistent Database
- Use BoltDB (already in use for Raft)
- Store LMS keys persistently in `hsm-data/keys.db`
- Schema: `key_id -> LMSKeyData` (serialized)

### 2. Update LMSKey Structure
Current structure only stores metadata. Need to add:
```go
type LMSKey struct {
    KeyID      string
    Index      uint64
    Created    time.Time
    PrivateKey []byte  // Serialized LMS private key
    PublicKey  []byte  // Serialized LMS public key
    Params     string  // LMS parameters (e.g., "LMS_SHA256_M32_H5")
}
```

### 3. Key Generation
- Use `hash-sigs` library to generate LMS key pair
- Store both private and public keys in DB
- Return public key to client (private key never leaves HSM server)

### 4. Message Signing
- Load LMS private key from DB
- Use `hash-sigs` to sign message
- Increment index after signing
- Update DB with new index
- Return signature to client

### 5. Integration Points
- `hsm_server/hsm_server.go`: Add DB field, update generateKey()
- `hsm_server/db.go`: New file for DB operations
- `hsm_server/lms.go`: New file for LMS operations (key gen, signing)
- `hsm_server/sign.go`: Update to use real LMS signing

## Dependencies
```bash
go get github.com/cisco/hash-sigs
```

## Testing
- Generate key -> verify public key format
- Sign message -> verify signature format
- Verify signature with public key
- Test index increment and persistence
