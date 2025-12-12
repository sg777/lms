# Why Signature Verification Succeeded in Tests Despite Inconsistent Keys

## The Critical Discovery

After analyzing the test code, here's what actually happened:

### Test Flow Analysis

**`TestExactCommandFlow` does this:**

1. **Loads keys from files:**
   ```go
   privKey, pubKey, err := LoadAttestationKeyPair()
   ```
   - Loads `privKey` from `keys/attestation_private_key.pem`
   - Loads `pubKey` from `keys/attestation_public_key.pem`
   - **These might be inconsistent**

2. **Signs with private key:**
   ```go
   signature, err := ecdsa.SignASN1(rand.Reader, privKey, hash[:])
   ```
   - Creates signature using `privKey` (from file)

3. **Puts public key in entry:**
   ```go
   PublicKey: base64.StdEncoding.EncodeToString(pubKeyBytes)
   ```
   - Uses `pubKey` (from file) in the entry

4. **FSM verifies using public key from entry:**
   ```go
   // FSM's verifySignature uses entry.PublicKey
   pubKeyBytes, err := base64.StdEncoding.DecodeString(entry.PublicKey)
   pubKey, _ := x509.ParsePKIXPublicKey(pubKeyBytes)
   ecdsa.VerifyASN1(pubKey, hash[:], sigBytes)
   ```
   - Verifies using `pubKey` from entry (which came from file)

## Why Tests Passed

**The tests passed because they used the SAME inconsistent keys for both signing and verification!**

Here's the key insight:

- **Test signs with:** `privKey` from file (keypair A)
- **Test verifies with:** `pubKey` from file (keypair B, different from A)
- **BUT:** The test loads BOTH keys from the SAME `LoadAttestationKeyPair()` call
- **The test doesn't check if they match!**

However, if the keys are truly inconsistent (private key A, public key B), then:
- Signature created with private key A
- Verification with public key B
- **Should FAIL**

## The Real Answer

**The tests passed because:**

1. **At the time tests ran, the keys in files might have been consistent** (or the test generated new keys)
2. **OR the old `LoadOrGenerateAttestationKeyPair()` function generated NEW consistent keys** if loading failed
3. **The test never actually verified that the keys from files are pairwise consistent**

## Why Production Failed

In production:
- HSM server loads keys from files (inconsistent)
- Signs with private key A
- Sends public key B in request
- Raft verifies with public key B
- **FAILS because signature was created with key A but verified with key B**

## The Missing Test

The test that was missing (and now added):
- `TestLoadAttestationKeyPair_Consistency` - Verifies that `privKey.PublicKey == pubKey`
- This would have caught the inconsistency immediately

## Conclusion

**Tests passed because they didn't verify key consistency.** They only verified that the crypto operations work with whatever keys were loaded, without checking if those keys actually form a valid key pair.

