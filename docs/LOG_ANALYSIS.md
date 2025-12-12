# Raft Log Analysis

## Summary

Node3 started as bootstrap, but **node1 was not running**. Node3 eventually became leader with node2's help, then node1 joined later.

## Timeline

### Phase 1: Node3 Trying to Become Leader (Lines 685-836)
- **Problem**: Node3 can't connect to node1 or node2
- **Errors**: `connection refused` to both 159.69.23.29:7000 and 159.69.23.30:7000
- **Status**: Node3 keeps trying to get votes but fails
- **Reason**: Node1 and Node2 are not running yet

### Phase 2: Node2 Comes Online (Line 837)
- Node2 sends pre-vote request
- Node3 rejects it because there's already a leader (node1 was leader before)
- This means node1 was running earlier, then went offline

### Phase 3: Node3 Becomes Leader (Lines 838-862)
- **Line 838**: Node3 detects heartbeat timeout (no leader)
- **Line 850**: Node2 grants pre-vote to node3
- **Line 857**: Node2 grants vote to node3
- **Line 858**: **Node3 wins election** (2 votes: node2 + node3)
- **Line 862**: "Node node3 is now the leader"

### Phase 4: Node3 Trying to Contact Node1 (Lines 863-939)
- **Problem**: Node1 is offline
- **Errors**: Continuous `connection refused` to 159.69.23.29:7000
- **Behavior**: Normal - leader tries to replicate to all followers
- **Impact**: Cluster still works with 2/3 nodes (majority)

### Phase 5: Node1 Comes Online (Line 940)
- **Line 940**: "pipelining replication: peer=node1"
- Node1 finally connects and starts receiving logs
- Cluster is now complete (3/3 nodes)

## Key Findings

### ✅ Normal Behavior
1. **Cluster works with 2/3 nodes**: Raft only needs majority (2 out of 3)
2. **Leader election succeeded**: Node3 got votes from node2 and itself
3. **Replication attempts**: Leader continuously tries to contact offline nodes (expected)
4. **Auto-recovery**: When node1 came online, it automatically joined

### ⚠️ Issues
1. **Node1 was offline**: Should have been running when node3 started
2. **Connection refused errors**: Expected when nodes are offline, but noisy in logs
3. **Startup order**: Nodes should start in sequence (bootstrap first, then others)

## Recommendations

### 1. Start Nodes in Correct Order
```bash
# Step 1: Start bootstrap node FIRST
./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap

# Step 2: Wait for "Node node3 is now the leader"

# Step 3: Start other nodes
./lms-service -id node1 -addr 159.69.23.29:7000
./lms-service -id node2 -addr 159.69.23.30:7000
```

### 2. Check Node Status Before Starting
```bash
# Verify ports are available
netstat -tlnp | grep 7000

# Check if other nodes are running
curl http://159.69.23.29:8080/health
curl http://159.69.23.30:8080/health
```

### 3. Reduce Log Noise (Optional)
The connection refused errors are normal but noisy. You could:
- Filter logs to show only important messages
- Add retry backoff (already happening)
- Accept that offline nodes generate errors

## Current Status

✅ **Cluster is working!**
- Node3 is leader
- Node2 is follower (connected)
- Node1 is follower (connected after coming online)
- All 3 nodes are now in the cluster

## Conclusion

The logs show **normal Raft behavior** when nodes start at different times. The cluster operated correctly with 2/3 nodes and automatically recovered when node1 joined. The "connection refused" errors are expected when nodes are offline and will stop once all nodes are running.

