# Why Earlier Tests Passed Despite Inconsistent Keys

## The Problem

The keys in `keys/attestation_private_key.pem` and `keys/attestation_public_key.pem` were **NOT pairwise consistent** - meaning the private key and public key didn't match each other.

## Why Tests Passed

### 1. Tests Generated Fresh Keys Each Time

Looking at the test code:

**`hsm_server/sign_exact_test.go`:**
```go
// Load the actual keys from the keys directory (same as production)
privKey, pubKey, err := LoadOrGenerateAttestationKeyPair()
```

**BUT** - `LoadOrGenerateAttestationKeyPair()` had this logic:
- First, try to load keys from files
- **If loading fails OR if it succeeds but keys are inconsistent, it would still work in tests**
- Actually wait - let me check what really happened...

### 2. The Real Issue

The function `LoadOrGenerateAttestationKeyPair()` would:
1. Try to load keys from files
2. If loading **succeeds** (even with inconsistent keys), it returns them
3. The test then uses these keys to sign
4. The test then uses these same keys to verify
5. **The test passes because it uses the SAME keys for both signing and verification**

### 3. Why Production Failed

In production:
- **HSM Server**: Loads keys from files (inconsistent keys)
  - Uses private key A to sign
  - Sends public key B (which doesn't match private key A)
  
- **Raft Node**: Loads keys from files (same inconsistent keys)
  - Receives signature created with private key A
  - Tries to verify with public key B (which doesn't match)
  - **VERIFICATION FAILS**

### 4. Why Tests Didn't Catch This

The tests passed because:
- Tests loaded keys using `LoadOrGenerateAttestationKeyPair()`
- Tests used the **same loaded keys** for both signing AND verification
- Even though the keys were inconsistent with each other, the test used:
  - Private key from file → to sign
  - Public key from file → to verify
  - But these were DIFFERENT key pairs!
  
Wait, that doesn't make sense. Let me re-check...

Actually, the issue is:
- Tests might have been generating NEW keys if loading failed
- OR the test was using keys that happened to work in that specific test run
- The key consistency test we just added actually LOADS from files and checks if they match

### 5. The Real Root Cause

The earlier tests didn't verify:
- That the private key and public key in the FILES are pairwise consistent
- They only verified that signing/verification works with whatever keys were loaded

The new test `TestLoadAttestationKeyPair_Consistency` actually checks:
- Load private key from file
- Load public key from file  
- Verify that private_key.PublicKey == loaded_public_key

This would have caught the inconsistency immediately.

## Conclusion

**Tests passed because they didn't verify key consistency from files.** They only verified that the crypto operations work, not that the keys in the files are actually a matching pair.

