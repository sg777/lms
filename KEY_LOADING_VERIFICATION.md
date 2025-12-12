# Attestation Key Loading Verification

## ✅ Confirmed: Keys are ONLY loaded from files, NEVER generated

### 1. HSM Server (`hsm_server/keys.go`)
- **Function**: `LoadAttestationKeyPair()`
- **Behavior**: ONLY loads keys from `./keys/attestation_private_key.pem` and `./keys/attestation_public_key.pem`
- **On Error**: Returns error immediately - NO generation fallback
- **Code**: Lines 20-31 - pure file loading only

### 2. HSM Server Initialization (`hsm_server/hsm_server.go`)
- **Function**: `NewHSMServer()`
- **Behavior**: Calls `LoadAttestationKeyPair()` - if it fails, returns error
- **No Fallback**: Line 33 - returns error if loading fails
- **No Generation**: No code path that generates keys

### 3. FSM Key Loading (`fsm/key_index_fsm.go`)
- **Function**: `loadAttestationPublicKey()`
- **Behavior**: ONLY loads public key from file
- **On Error**: Returns error - NO generation
- **Code**: Lines 53-79 - pure file loading only

### 4. Main Service (`main.go`)
- **Behavior**: Tries to load public key for FSM
- **Fallback**: If key loading fails, falls back to `HashChainFSM` (no signature verification)
- **No Generation**: Does NOT generate keys, just disables signature verification

### 5. Test Files
- **Note**: Test files (`*_test.go`) use `ecdsa.GenerateKey()` to create **test keys** for unit tests
- **This is expected**: Tests need to generate their own keys for testing
- **Not used in production**: These are isolated to test files only

### 6. No File Writing Code
- **Verified**: No `os.WriteFile`, `ioutil.WriteFile`, or any file creation for keys
- **Confirmed**: Keys are read-only in production code

## Summary

✅ **Attestation keys are ONLY loaded from committed files**
✅ **No generation code exists in production paths**
✅ **If keys are missing, the system returns errors (HSM server) or falls back to hash-chain only (Raft service)**
✅ **Keys must be committed to git and pulled on all nodes**

## Files to Verify on Each Node

After `git pull`, verify keys exist:
```bash
ls -la keys/attestation_*.pem
cat keys/attestation_public_key.pem
```

Both files should exist and be identical across all nodes.

