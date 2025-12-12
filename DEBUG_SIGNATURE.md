# Debug Signature Verification

## Issue
Tests pass but production fails with "ECDSA verify returned false"

## Possible Causes

1. **Raft nodes not rebuilt** - Old code still running
2. **Data format mismatch** - Different format between signing and verification
3. **Key mismatch** - Different keys being used

## Check These:

1. **Rebuild all Raft nodes:**
   ```bash
   # On each Raft node
   cd /root/lms
   git pull
   go build -o lms-service ./main.go
   # Restart the service
   ```

2. **Verify data format matches:**
   - Signing: `fmt.Sprintf("%s:%d", keyID, index)` in `hsm_server/sign.go:83`
   - Verification: `fmt.Sprintf("%s:%d", entry.KeyID, entry.Index)` in `fsm/key_index_fsm.go:120`
   - These MUST match exactly

3. **Check keys are the same:**
   - HSM server uses keys from `./keys/` directory
   - Raft nodes should load public key from `./keys/attestation_public_key.pem` (optional, now uses key from request)

## Quick Fix Test

Run this to verify the exact data format:
```bash
go run -c 'package main; import "fmt"; func main() { fmt.Println(fmt.Sprintf("%s:%d", "lms_key_1", 0)) }'
# Should output: lms_key_1:0
```

