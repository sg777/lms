package lms_wrapper

/*
#cgo CFLAGS: -I${SRCDIR}/../native/hash-sigs
#cgo LDFLAGS: ${SRCDIR}/../native/hash-sigs/hss_lib_thread.a -lcrypto -lpthread
#include "hss.h"
#include "hss_verify.h"
#include "hss_common.h"
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// Global variable to store generated private key
unsigned char *g_privkey_buffer = NULL;
size_t g_privkey_len = 0;

// Random function for key generation
bool go_generate_random(void *output, size_t length) {
    FILE *f = fopen("/dev/urandom", "r");
    if (!f) return false;
    size_t read = fread(output, 1, length, f);
    fclose(f);
    return (read == length);
}

// Update private key function - stores it in global buffer
bool go_update_private_key(unsigned char *private_key, size_t len_private_key, void *context) {
    if (g_privkey_buffer) free(g_privkey_buffer);
    g_privkey_buffer = (unsigned char *)malloc(len_private_key);
    if (!g_privkey_buffer) return false;
    memcpy(g_privkey_buffer, private_key, len_private_key);
    g_privkey_len = len_private_key;
    return true;
}

// Read private key function for loading - context points to Go byte slice
bool go_read_private_key_from_context(unsigned char *private_key, size_t len_private_key, void *context) {
    if (!context) return false;
    unsigned char *go_privkey = (unsigned char *)context;
    memcpy(private_key, go_privkey, len_private_key);
    return true;
}

// Update private key during signing - updates the context (Go byte slice)
bool go_update_private_key_during_sign(unsigned char *private_key, size_t len_private_key, void *context) {
    if (!context) return false;
    unsigned char *go_privkey = (unsigned char *)context;
    memcpy(go_privkey, private_key, len_private_key);
    return true;
}
*/
import "C"
import (
	"errors"
	"sync"
	"unsafe"
)

// GenerateKeyPair generates a new HSS/LMS key pair
// levels: number of levels (typically 1 for LMS, 1-8 for HSS)
// lmType: LMS parameter set array (one per level) - values like 5 for LMS_SHA256_M32_H5
// otsType: OTS parameter set array (one per level) - values like 1 for LMOTS_SHA256_N32_W1
func GenerateKeyPair(levels int, lmType []int, otsType []int) ([]byte, []byte, error) {
	if len(lmType) != levels || len(otsType) != levels {
		return nil, nil, errors.New("parameter arrays must match levels")
	}
	
	// Convert Go slices to C arrays (param_set_t is uint_fast32_t = unsigned long)
	cLmType := make([]C.ulong, len(lmType))
	cOtsType := make([]C.ulong, len(otsType))
	for i, v := range lmType {
		cLmType[i] = C.ulong(v)
	}
	for i, v := range otsType {
		cOtsType[i] = C.ulong(v)
	}
	
	// Get expected key lengths
	pubKeyLen := C.hss_get_public_key_len(C.uint(levels), &cLmType[0], &cOtsType[0])
	if pubKeyLen <= 0 {
		return nil, nil, errors.New("invalid parameter set")
	}
	
	// Allocate buffers
	pubKeyBuf := make([]byte, pubKeyLen)
	
	// Call C function
	success := C.hss_generate_private_key(
		(*[0]byte)(C.go_generate_random),        // random function
		C.uint(levels),                          // levels
		&cLmType[0],                            // lm_type (param_set_t*)
		&cOtsType[0],                            // ots_type (param_set_t*)
		(*[0]byte)(C.go_update_private_key),    // update function
		nil,                                     // context
		(*C.uchar)(unsafe.Pointer(&pubKeyBuf[0])), // public_key
		C.size_t(pubKeyLen),                     // len_public_key
		nil,                                     // aux_data
		0,                                       // len_aux_data
		nil,                                     // info
	)
	
	if !success {
		return nil, nil, errors.New("failed to generate HSS key pair")
	}
	
	// Get private key from global buffer
	if C.g_privkey_buffer == nil || C.g_privkey_len == 0 {
		return nil, nil, errors.New("private key not stored")
	}
	
	privKey := C.GoBytes(unsafe.Pointer(C.g_privkey_buffer), C.int(C.g_privkey_len))
	
	return privKey, pubKeyBuf, nil
}

// WorkingKey represents a loaded HSS/LMS working key for signing
type WorkingKey struct {
	key        *C.struct_hss_working_key
	privKey    []byte // Keep reference to private key for updates
	levels     int
	lmType     []int
	otsType    []int
	mu         sync.Mutex // Protect concurrent access
}

// LoadWorkingKey loads a private key into a working key structure for signing
// privKey: the private key bytes (from GenerateKeyPair)
// levels, lmType, otsType: same parameters used to generate the key
// memoryTarget: memory budget (0 = minimal memory, higher = faster signing)
func LoadWorkingKey(privKey []byte, levels int, lmType []int, otsType []int, memoryTarget int) (*WorkingKey, error) {
	if len(privKey) == 0 {
		return nil, errors.New("private key is empty")
	}
	if len(lmType) != levels || len(otsType) != levels {
		return nil, errors.New("parameter arrays must match levels")
	}
	
	// Convert Go slices to C arrays
	cLmType := make([]C.ulong, len(lmType))
	cOtsType := make([]C.ulong, len(otsType))
	for i, v := range lmType {
		cLmType[i] = C.ulong(v)
	}
	for i, v := range otsType {
		cOtsType[i] = C.ulong(v)
	}
	
	// Make a copy of private key for the working key to update
	privKeyCopy := make([]byte, len(privKey))
	copy(privKeyCopy, privKey)
	
	// Load the working key
	// Use NULL read function and pass privKey directly as context
	// When read_private_key is NULL, context is treated as the private key buffer
	workingKey := C.hss_load_private_key(
		(*[0]byte)(C.go_read_private_key_from_context), // read function
		unsafe.Pointer(&privKeyCopy[0]),                // context (private key buffer)
		C.size_t(memoryTarget),                        // memory target
		nil,                                           // aux_data (optional)
		0,                                             // len_aux_data
		nil,                                           // info
	)
	
	if workingKey == nil {
		return nil, errors.New("failed to load working key")
	}
	
	return &WorkingKey{
		key:     workingKey,
		privKey: privKeyCopy,
		levels:  levels,
		lmType:  lmType,
		otsType: otsType,
	}, nil
}

// GenerateSignature signs a message using the working key
// The private key state is updated after signing (stateful signature scheme)
func (wk *WorkingKey) GenerateSignature(message []byte) ([]byte, error) {
	wk.mu.Lock()
	defer wk.mu.Unlock()
	
	if wk.key == nil {
		return nil, errors.New("working key is not loaded")
	}
	
	if len(message) == 0 {
		return nil, errors.New("message is empty")
	}
	
	// Convert Go slices to C arrays for signature length calculation
	cLmType := make([]C.ulong, len(wk.lmType))
	cOtsType := make([]C.ulong, len(wk.otsType))
	for i, v := range wk.lmType {
		cLmType[i] = C.ulong(v)
	}
	for i, v := range wk.otsType {
		cOtsType[i] = C.ulong(v)
	}
	
	// Get expected signature length
	sigLen := C.hss_get_signature_len(C.uint(wk.levels), &cLmType[0], &cOtsType[0])
	if sigLen <= 0 {
		return nil, errors.New("failed to get signature length")
	}
	
	// Allocate signature buffer
	sigBuf := make([]byte, sigLen)
	
	// Generate signature
	success := C.hss_generate_signature(
		wk.key,                                    // working_key
		(*[0]byte)(C.go_update_private_key_during_sign), // update function
		unsafe.Pointer(&wk.privKey[0]),            // context (private key to update)
		unsafe.Pointer(&message[0]),               // message
		C.size_t(len(message)),                    // message_len
		(*C.uchar)(unsafe.Pointer(&sigBuf[0])),    // signature
		C.size_t(sigLen),                          // signature_len
		nil,                                       // info
	)
	
	if !success {
		return nil, errors.New("failed to generate signature")
	}
	
	return sigBuf, nil
}

// Free releases the working key resources
func (wk *WorkingKey) Free() {
	wk.mu.Lock()
	defer wk.mu.Unlock()
	
	if wk.key != nil {
		C.hss_free_working_key(wk.key)
		wk.key = nil
	}
}

// GetPrivateKey returns the current state of the private key (updated after signing)
func (wk *WorkingKey) GetPrivateKey() []byte {
	wk.mu.Lock()
	defer wk.mu.Unlock()
	
	privKeyCopy := make([]byte, len(wk.privKey))
	copy(privKeyCopy, wk.privKey)
	return privKeyCopy
}

// VerifySignature verifies an HSS/LMS signature
func VerifySignature(publicKey []byte, message []byte, signature []byte) (bool, error) {
	if len(publicKey) == 0 || len(message) == 0 || len(signature) == 0 {
		return false, errors.New("empty input")
	}
	
	result := C.hss_validate_signature(
		(*C.uchar)(unsafe.Pointer(&publicKey[0])),
		unsafe.Pointer(&message[0]),
		C.size_t(len(message)),
		(*C.uchar)(unsafe.Pointer(&signature[0])),
		C.size_t(len(signature)),
		nil, // info
	)
	
	return bool(result), nil
}
