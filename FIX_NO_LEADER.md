# Fix: No Leader Election After Node3 Disconnect

## Problem

When node3 (leader) is disconnected, node1 and node2 don't elect a new leader.

## Root Causes

### 1. Node1 and Node2 Can't Communicate ⚠️ MOST LIKELY
- For a new leader to be elected, node1 and node2 need to communicate
- They need to exchange votes to form a quorum (2 out of 3)
- **Check**: Can node1 reach node2 on port 7000?

### 2. Node1 Not Running
- If only node2 is running, it can't form a quorum alone
- **Check**: Is node1 actually running?

### 3. Raft Timeout Too Long (FIXED)
- ✅ **FIXED**: Reduced timeouts to 500ms for faster failover
- Rebuild and restart nodes with new binary

## Solution Steps

### Step 1: Verify Both Nodes Are Running

```bash
# Check node1
curl http://159.69.23.29:8080/health

# Check node2
curl http://159.69.23.30:8080/health
```

**Both should respond!** If one doesn't, start it.

### Step 2: Test Network Connectivity

```bash
# From node1, test connection to node2
telnet 159.69.23.30 7000
# Should connect (Ctrl+] then 'quit' to exit)

# From node2, test connection to node1
telnet 159.69.23.29 7000
# Should connect
```

**If connection fails**: Firewall or network issue!

### Step 3: Rebuild with Faster Timeouts

```bash
cd /root/lms
go build -o lms-service .
```

The new binary has:
- Heartbeat timeout: 500ms (was ~1s)
- Election timeout: 500ms (was ~1s)
- Leader lease timeout: 500ms

### Step 4: Restart Nodes with New Binary

```bash
# Stop all nodes (Ctrl+C)

# Restart node1
./lms-service -id node1 -addr 159.69.23.29:7000

# Restart node2
./lms-service -id node2 -addr 159.69.23.30:7000
```

### Step 5: Test Failover Again

1. Kill node3 (if it was running)
2. Wait 1-2 seconds
3. Check logs on node1 and node2
4. One should become leader

## Expected Logs When Working

On the node that becomes leader:
```
[WARN] raft: heartbeat timeout reached, starting election
[INFO] raft: entering candidate state
[INFO] raft: election won
[INFO] raft: entering leader state
Node node1 is now the leader  (or node2)
```

## Quick Diagnostic

Run the diagnostic script:
```bash
./DIAGNOSE_FAILOVER.sh
```

This will check:
- If nodes are running
- Current leader status
- Network connectivity hints

## Most Common Issue

**Node1 and Node2 can't talk to each other!**

Check:
- Firewall rules (port 7000 must be open)
- Network connectivity
- Both nodes actually running

