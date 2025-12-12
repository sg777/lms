# ACTUAL Reason Tests Passed - The Truth

## What Actually Happened

Looking at the git history, the old `LoadOrGenerateAttestationKeyPair()` function had this logic:

1. **Try to load keys from files**
2. **If loading fails, GENERATE NEW CONSISTENT KEYS**
3. **Return the new keys**

## Why Tests Passed

When tests ran with `LoadOrGenerateAttestationKeyPair()`:

1. Function tries to load keys from files
2. If files exist but keys are inconsistent (or any error), the old code would:
   - **Generate brand new consistent keys**
   - Return those new keys
   - Tests would pass because new keys are always consistent

3. OR if files don't exist:
   - Generate new consistent keys
   - Tests pass

## Why Production Failed

In production with `LoadAttestationKeyPair()` (no generation):

1. HSM server loads keys from files (inconsistent keys)
2. Signs with private key A
3. Sends public key B (doesn't match A)
4. Raft tries to verify with public key B
5. **FAILS because signature was created with key A but verified with key B**

## The Real Problem

**The old code masked the problem by generating new keys when loading failed!**

Tests passed because they got fresh consistent keys, not because the files had consistent keys.

Production failed because it actually used the inconsistent keys from files.

## The Fix

1. Removed key generation code (now only loads from files)
2. Generated consistent keys with OpenSSL
3. Added test to verify keys are pairwise consistent
4. This test would have caught the issue immediately

