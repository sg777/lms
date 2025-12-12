# LMS Wrapper API Documentation

## Overview

The `lms_wrapper` package provides Go bindings to Cisco's hash-sigs C library for LMS/HSS signature operations.

## Functions

### GenerateKeyPair

Generates a new LMS/HSS key pair.

```go
func GenerateKeyPair(levels int, lmType []int, otsType []int) ([]byte, []byte, error)
```

**Parameters:**
- `levels`: Number of levels (1 for LMS, 1-8 for HSS)
- `lmType`: LMS parameter set array (e.g., `[]int{5}` for LMS_SHA256_M32_H5)
- `otsType`: OTS parameter set array (e.g., `[]int{1}` for LMOTS_SHA256_N32_W1)

**Returns:**
- `[]byte`: Private key
- `[]byte`: Public key
- `error`: Error if generation fails

**Example:**
```go
privKey, pubKey, err := lms_wrapper.GenerateKeyPair(
    1,        // Single level (LMS, not HSS)
    []int{5}, // LMS_SHA256_M32_H5
    []int{1}, // LMOTS_SHA256_N32_W1
)
```

### LoadWorkingKey

Loads a private key into a working key structure for signing.

```go
func LoadWorkingKey(privKey []byte, levels int, lmType []int, otsType []int, memoryTarget int) (*WorkingKey, error)
```

**Parameters:**
- `privKey`: Private key bytes (from `GenerateKeyPair`)
- `levels`, `lmType`, `otsType`: Same parameters used to generate the key
- `memoryTarget`: Memory budget (0 = minimal memory, higher = faster signing)

**Returns:**
- `*WorkingKey`: Working key for signing
- `error`: Error if loading fails

**Example:**
```go
workingKey, err := lms_wrapper.LoadWorkingKey(privKey, 1, []int{5}, []int{1}, 0)
defer workingKey.Free() // Always free when done
```

### WorkingKey.GenerateSignature

Signs a message using the loaded working key. **Note:** This is stateful - the private key state is updated after each signature.

```go
func (wk *WorkingKey) GenerateSignature(message []byte) ([]byte, error)
```

**Parameters:**
- `message`: Message to sign

**Returns:**
- `[]byte`: Signature
- `error`: Error if signing fails

**Example:**
```go
signature, err := workingKey.GenerateSignature([]byte("Hello, World!"))
```

### WorkingKey.Free

Releases the working key resources. **Always call this when done with a working key.**

```go
func (wk *WorkingKey) Free()
```

### WorkingKey.GetPrivateKey

Returns the current state of the private key (updated after each signature).

```go
func (wk *WorkingKey) GetPrivateKey() []byte
```

**Note:** LMS is stateful - the private key changes after each signature. Save the updated private key if you need to resume signing later.

### VerifySignature

Verifies an LMS/HSS signature.

```go
func VerifySignature(publicKey []byte, message []byte, signature []byte) (bool, error)
```

**Parameters:**
- `publicKey`: Public key
- `message`: Original message
- `signature`: Signature to verify

**Returns:**
- `bool`: `true` if signature is valid, `false` otherwise
- `error`: Error if verification fails

**Example:**
```go
valid, err := lms_wrapper.VerifySignature(pubKey, message, signature)
if valid {
    fmt.Println("Signature is valid!")
}
```

## Complete Example

```go
package main

import (
    "fmt"
    "github.com/verifiable-state-chains/lms/lms_wrapper"
)

func main() {
    // 1. Generate key pair
    privKey, pubKey, err := lms_wrapper.GenerateKeyPair(1, []int{5}, []int{1})
    if err != nil {
        panic(err)
    }
    
    // 2. Load working key
    workingKey, err := lms_wrapper.LoadWorkingKey(privKey, 1, []int{5}, []int{1}, 0)
    if err != nil {
        panic(err)
    }
    defer workingKey.Free()
    
    // 3. Sign message
    message := []byte("Hello, LMS!")
    signature, err := workingKey.GenerateSignature(message)
    if err != nil {
        panic(err)
    }
    
    // 4. Verify signature
    valid, err := lms_wrapper.VerifySignature(pubKey, message, signature)
    if err != nil {
        panic(err)
    }
    
    if valid {
        fmt.Println("Signature verified!")
    }
    
    // 5. Get updated private key (state changed after signing)
    updatedPrivKey := workingKey.GetPrivateKey()
    // Save updatedPrivKey if you need to resume signing later
}
```

## Parameter Sets

Common parameter sets:

- **LMS Types:**
  - `5`: LMS_SHA256_M32_H5 (height 5, ~32 signatures)
  - `6`: LMS_SHA256_M32_H10 (height 10, ~1024 signatures)
  - `7`: LMS_SHA256_M32_H15 (height 15, ~32768 signatures)
  - `8`: LMS_SHA256_M32_H20 (height 20, ~1M signatures)
  - `9`: LMS_SHA256_M32_H25 (height 25, ~33M signatures)

- **OTS Types:**
  - `1`: LMOTS_SHA256_N32_W1 (Winternitz parameter 1)
  - `2`: LMOTS_SHA256_N32_W2 (Winternitz parameter 2)
  - `3`: LMOTS_SHA256_N32_W4 (Winternitz parameter 4)
  - `4`: LMOTS_SHA256_N32_W8 (Winternitz parameter 8)

## Thread Safety

- `WorkingKey` methods are thread-safe (protected by mutex)
- Multiple `WorkingKey` instances can be used concurrently
- `GenerateKeyPair` and `VerifySignature` are safe for concurrent use

## Memory Management

- Always call `WorkingKey.Free()` when done with a working key
- Private keys returned by `GetPrivateKey()` are copies - safe to use after `Free()`
