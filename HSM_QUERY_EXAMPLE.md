# How HSMs Query Latest State

## Method 1: Using HSM Client Library (Recommended)

```go
package main

import (
    "fmt"
    "github.com/verifiable-state-chains/lms/client"
)

func main() {
    // Step 1: Create HSM client
    endpoints := []string{
        "http://159.69.23.29:8080",
        "http://159.69.23.30:8080",
        "http://159.69.23.31:8080",
    }
    
    hsmClient := client.NewHSMClient(endpoints, "hsm-1")
    
    // Step 2: Query latest state
    attestation, raftIndex, raftTerm, err := hsmClient.GetLatestHead()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    if attestation == nil {
        fmt.Println("Chain is empty (genesis)")
        return
    }
    
    // Step 3: Extract state from attestation
    payload, _ := attestation.GetChainedPayload()
    fmt.Printf("Latest LMS Index: %d\n", payload.LMSIndex)
    fmt.Printf("Latest Sequence: %d\n", payload.SequenceNumber)
    fmt.Printf("Raft Index: %d\n", raftIndex)
    fmt.Printf("Raft Term: %d\n", raftTerm)
}
```

## Method 2: Using Protocol (Easier)

```go
package main

import (
    "fmt"
    "github.com/verifiable-state-chains/lms/client"
)

func main() {
    endpoints := []string{
        "http://159.69.23.29:8080",
        "http://159.69.23.30:8080",
        "http://159.69.23.31:8080",
    }
    
    genesisHash := "lms_genesis_hash_verifiable_state_chains"
    hsmClient := client.NewHSMClient(endpoints, "hsm-1")
    protocol := client.NewHSMProtocol(hsmClient, genesisHash)
    
    // Sync state (queries latest head automatically)
    err := protocol.SyncState()
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Get state
    state := protocol.GetState()
    fmt.Printf("Current LMS Index: %d\n", state.CurrentLMSIndex)
    fmt.Printf("Current Sequence: %d\n", state.SequenceNumber)
    fmt.Printf("Last Raft Index: %d\n", state.LastRaftIndex)
}
```

## Method 3: Direct HTTP API Call

```bash
# Query latest head
curl http://159.69.23.29:8080/latest-head

# Response:
{
  "success": true,
  "attestation": {
    "attestation_response": {
      "data": {
        "value": "...",
        "encoding": "base64"
      },
      ...
    }
  },
  "raft_index": 5,
  "raft_term": 1
}
```

## What Happens Under the Hood:

1. **HSM calls** `client.GetLatestHead()` or `protocol.SyncState()`
2. **Client makes HTTP GET** to `/latest-head` endpoint
3. **Service queries FSM** for latest attestation
4. **Service returns** the attestation + Raft metadata
5. **HSM extracts** LMS index, sequence number, etc. from the attestation

## The API Endpoint:

**Location**: `service/api.go` line 117-163

```go
// handleLatestHead handles requests for the latest attestation head
func (s *APIServer) handleLatestHead(w http.ResponseWriter, r *http.Request) {
    // Get latest attestation from FSM
    attestation, err := s.fsm.GetLatestAttestation()
    
    // Return it as JSON
    response := models.GetLatestHeadResponse{
        Success:     true,
        Attestation: attestation,
        RaftIndex:   raftIndex,
        RaftTerm:    raftTerm,
    }
    json.NewEncoder(w).Encode(response)
}
```

## Complete Workflow:

```go
// 1. HSM connects
client := client.NewHSMClient(endpoints, "hsm-1")
protocol := client.NewHSMProtocol(client, genesisHash)

// 2. HSM queries latest state
protocol.SyncState()  // Calls GetLatestHead() internally

// 3. HSM gets current state
state := protocol.GetState()
// state.CurrentLMSIndex = latest LMS index
// state.SequenceNumber = latest sequence number
// state.LastAttestation = latest attestation

// 4. HSM uses this to create next attestation
nextIndex := protocol.GetNextUsableIndex()  // Returns CurrentLMSIndex + 1
```

## That's It!

The HSM client library handles all the HTTP calls, error handling, and leader forwarding automatically.

