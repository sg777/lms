package explorer

import (
	"os"
	"strconv"

	"github.com/verifiable-state-chains/lms/blockchain"
)

// verusRPCConfig returns hardcoded RPC config
func verusRPCConfig() (url, user, pass string) {
	url = "http://127.0.0.1:22778"
	user = "user1172159772"
	pass = "pass03465d081d1dfd2b74a2b5de27063f44f6843c64bcd63a6797915eb0ffa25707da"
	return
}

func verusIdentityName() string {
	return "sg777z.chips.vrsc@"
}

// getBootstrapBlockHeight returns the bootstrap block height from environment variable
// Commits before this block height will be ignored
// Set LMS_BOOTSTRAP_BLOCK_HEIGHT environment variable to enable filtering
// Returns 0 if not set (no filtering)
func getBootstrapBlockHeight() int64 {
	envVal := os.Getenv("LMS_BOOTSTRAP_BLOCK_HEIGHT")
	if envVal == "" {
		return 0 // No filtering
	}
	height, err := strconv.ParseInt(envVal, 10, 64)
	if err != nil {
		return 0 // Invalid value, no filtering
	}
	return height
}

// Convenience helper to build a Verus client
func newVerusClientFromEnv() *blockchain.VerusClient {
	url, user, pass := verusRPCConfig()
	return blockchain.NewVerusClient(url, user, pass)
}
