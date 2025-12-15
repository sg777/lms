package explorer

import (
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

// Convenience helper to build a Verus client
func newVerusClientFromEnv() *blockchain.VerusClient {
	url, user, pass := verusRPCConfig()
	return blockchain.NewVerusClient(url, user, pass)
}
