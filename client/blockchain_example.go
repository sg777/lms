package client

import (
	"crypto/sha256"
	"fmt"
	"log"

	"github.com/verifiable-state-chains/lms/blockchain"
)

// ExampleHSMProtocolWithBlockchain demonstrates HSM protocol with blockchain fallback
func ExampleHSMProtocolWithBlockchain() {
	// Step 1: Create HSM client
	serviceEndpoints := []string{
		"http://159.69.23.29:8080",
		"http://159.69.23.30:8080",
		"http://159.69.23.31:8080",
	}
	
	hsmID := "hsm-partition-001"
	hsmClient := NewHSMClient(serviceEndpoints, hsmID)
	
	// Step 2: Create blockchain client (optional, for fallback)
	verusClient := blockchain.NewVerusClient(
		"http://127.0.0.1:22778",
		"user1172159772",
		"pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da",
	)
	
	// Step 3: Compute pubkey_hash from LMS public key
	// In production, this comes from the HSM's LMS key
	lmsPublicKey := []byte("example-lms-public-key")
	pubkeyHashHex := computePubkeyHashHex(lmsPublicKey) // SHA256 hash as hex
	
	// Step 4: Configure blockchain fallback (optional)
	blockchainConfig := &BlockchainConfig{
		Enabled:       true,
		VerusClient:   verusClient,
		IdentityName:  "sg777z.chips.vrsc@",
		PubkeyHashHex: pubkeyHashHex,
	}
	
	// Step 5: Create protocol with blockchain fallback
	genesisHash := ComputeGenesisHash(lmsPublicKey, []byte("system-bundle"))
	protocol := NewHSMProtocol(hsmClient, genesisHash, blockchainConfig)
	
	// Step 6: Use protocol as normal
	// If Raft times out, it will automatically fall back to blockchain
	log.Printf("HSM Protocol initialized with blockchain fallback enabled")
	log.Printf("Pubkey hash: %s", pubkeyHashHex)
	
	_ = protocol // Use protocol for commits...
}

// Helper function to compute pubkey_hash as hex string
func computePubkeyHashHex(publicKey []byte) string {
	hash := sha256.Sum256(publicKey)
	return fmt.Sprintf("%x", hash)
}

