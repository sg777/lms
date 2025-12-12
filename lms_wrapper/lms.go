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
static unsigned char *g_privkey_buffer = NULL;
static size_t g_privkey_len = 0;

// Random function for key generation
static bool go_generate_random(void *output, size_t length) {
    FILE *f = fopen("/dev/urandom", "r");
    if (!f) return false;
    size_t read = fread(output, 1, length, f);
    fclose(f);
    return (read == length);
}

// Update private key function - stores it in global buffer
static bool go_update_private_key(unsigned char *private_key, size_t len_private_key, void *context) {
    if (g_privkey_buffer) free(g_privkey_buffer);
    g_privkey_buffer = (unsigned char *)malloc(len_private_key);
    if (!g_privkey_buffer) return false;
    memcpy(g_privkey_buffer, private_key, len_private_key);
    g_privkey_len = len_private_key;
    return true;
}

// Read private key function for loading
static bool go_read_private_key(unsigned char *private_key, size_t len_private_key, void *context) {
    if (!g_privkey_buffer || g_privkey_len != len_private_key) return false;
    memcpy(private_key, g_privkey_buffer, len_private_key);
    return true;
}
*/
import "C"
import (
	"errors"
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

// VerifySignature verifies an HSS signature
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
