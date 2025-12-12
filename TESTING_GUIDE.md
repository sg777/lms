# Testing Guide

## Module 1: Data Models Testing

### Prerequisites
- Go 1.21+ installed
- All dependencies downloaded (`go mod tidy`)

### Running Unit Tests

```bash
cd /root/lms

# Run all model tests
go test ./models -v

# Run with coverage
go test ./models -cover

# Run specific test
go test ./models -v -run TestChainedPayloadSerialization
```

### Expected Output

All 5 tests should pass:
```
=== RUN   TestChainedPayloadSerialization
--- PASS: TestChainedPayloadSerialization (0.00s)
=== RUN   TestAttestationResponseChainedPayload
--- PASS: TestAttestationResponseChainedPayload (0.00s)
=== RUN   TestAttestationResponseHash
--- PASS: TestAttestationResponseHash (0.00s)
=== RUN   TestLogEntrySerialization
--- PASS: TestLogEntrySerialization (0.00s)
=== RUN   TestCreateGenesisPayload
--- PASS: TestCreateGenesisPayload (0.00s)
PASS
ok  	github.com/verifiable-state-chains/lms/models	0.007s
```

### Manual Testing

You can also create a simple test program:

```bash
# Create test file
cat > test_models.go << 'EOF'
package main

import (
	"fmt"
	"time"
	"github.com/verifiable-state-chains/lms/models"
)

func main() {
	// Create a chained payload
	payload := &models.ChainedPayload{
		PreviousHash:   "genesis_hash_123",
		LMSIndex:       1,
		MessageSigned:  "message_hash_456",
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Metadata:       "test",
	}

	// Create attestation response
	ar := &models.AttestationResponse{}
	ar.AttestationResponse.Policy.Value = "LMS_ATTEST_POLICY"
	ar.AttestationResponse.Policy.Algorithm = "PS256"
	ar.SetChainedPayload(payload)

	// Compute hash
	hash, _ := ar.ComputeHash()
	fmt.Printf("Attestation hash: %s\n", hash)
	fmt.Printf("LMS Index: %d\n", payload.LMSIndex)
	fmt.Println("✅ Models working correctly!")
}
EOF

# Run it
go run test_models.go
```

## Node Configuration

For future modules (Raft cluster setup), you'll use these 3 nodes:

- **Node 1**: `159.69.23.29:7000`
- **Node 2**: `159.69.23.30:7000`
- **Node 3**: `159.69.23.31:7000`

## Git Push Authentication

If you need to push to GitHub, you'll need to authenticate:

### Option 1: SSH (Recommended)
```bash
# Change remote to SSH
git remote set-url origin git@github.com:sg777/lms.git

# Then push
git push -u origin main
```

### Option 2: Personal Access Token
```bash
# Use token as password when prompted
git push -u origin main
# Username: sg777
# Password: <your_github_token>
```

### Option 3: Configure Git Credentials
```bash
git config --global credential.helper store
git push -u origin main
# Enter credentials once, they'll be saved
```

