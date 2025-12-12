# Simple Explanation - What This System Does

## The Problem It Solves

**LMS (Leighton-Micali Signatures) need to track which index was used last.**
- If you reuse an index, it's a security problem
- Multiple HSMs need to coordinate
- Need to know: "What's the last index that was used?"

## The Solution

**A service that stores the "last used index" and all HSMs check with it.**

## How It Works (Step by Step)

### Step 1: Start the Service

You run `lms-service` on 3 computers (nodes):

```bash
# Node 1
./lms-service -id node1 -addr 159.69.23.29:7000

# Node 2  
./lms-service -id node2 -addr 159.69.23.30:7000

# Node 3 (bootstrap - start this first)
./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
```

**What this does:** Creates a cluster that stores attestations. If one node dies, others keep working.

### Step 2: HSM Wants to Sign Something

When an HSM needs to sign a message:

1. **HSM asks service: "What's the last index you know about?"**
   ```go
   client := client.NewHSMClient(endpoints, "hsm-1")
   protocol := client.NewHSMProtocol(client, genesisHash)
   protocol.SyncState()  // Gets latest index from service
   ```

2. **Service responds: "Last index was 5"**

3. **HSM says: "OK, I'll use index 6"**
   ```go
   nextIndex := protocol.GetNextUsableIndex()  // Returns 6
   ```

4. **HSM creates attestation with index 6**

5. **HSM sends attestation to service: "Here's my attestation with index 6"**
   ```go
   protocol.CommitAttestation(attestation, timeout)
   ```

6. **Service stores it and tells all nodes: "Index 6 is now used"**

7. **All HSMs now know index 6 is used**

## What Gets Stored

Each attestation contains:
- **LMS Index**: Which index was used (0, 1, 2, 3, ...)
- **Previous Hash**: Hash of the previous attestation (creates a chain)
- **Sequence Number**: Just a counter (1, 2, 3, ...)
- **Message Hash**: What was signed
- **Signature**: HSM's signature
- **Certificate**: HSM's certificate

## The Hash Chain

Each attestation links to the previous one:

```
Genesis → Hash(Genesis) → Attestation1 → Hash(Attestation1) → Attestation2 → ...
```

This makes it impossible to tamper with the history.

## Real Example

```go
// HSM code (what a real HSM would do)

// 1. Connect to service
client := client.NewHSMClient([]string{
    "http://159.69.23.29:8080",
    "http://159.69.23.30:8080", 
    "http://159.69.23.31:8080",
}, "hsm-1")

protocol := client.NewHSMProtocol(client, genesisHash)

// 2. Ask: "What's the last index?"
protocol.SyncState()
state := protocol.GetState()
fmt.Printf("Last index was: %d\n", state.CurrentLMSIndex)

// 3. Use next index
nextIndex := protocol.GetNextUsableIndex()  // e.g., returns 5

// 4. Create attestation
payload := protocol.CreateAttestationPayload(nextIndex, messageHash, ...)
attestation := protocol.CreateAttestationResponse(payload, ...)

// 5. Send to service
protocol.CommitAttestation(attestation, timeout)

// Done! Index 5 is now stored in the service
```

## What You Can Do Right Now

### 1. Start the Service

```bash
./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
```

### 2. Check It's Working

```bash
curl http://159.69.23.31:8080/health
```

### 3. Query Latest State

```bash
curl http://159.69.23.31:8080/latest-head
```

Returns the latest attestation (or empty if none yet).

### 4. Submit an Attestation

Use the HSM client library or send HTTP POST to `/propose`.

## The Files

- **`main.go`** - The service that runs
- **`service/api.go`** - HTTP API (the endpoints HSMs call)
- **`fsm/hashchain_fsm.go`** - Stores attestations in memory
- **`client/`** - Library for HSMs to use
- **`simulator/`** - Just for testing (not needed for real HSMs)

## That's It!

**The service = A shared database that stores "which LMS index was used last"**

**HSMs = Ask the service, then tell it when they use a new index**

Nothing more complicated than that.

