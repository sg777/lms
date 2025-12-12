# Testing Leader Failover

## How Raft Failover Works

When the leader (node3) is terminated, the remaining nodes automatically elect a new leader.

## Test Steps

### 1. Current State
- Node3 is leader
- Node1 and Node2 are followers
- All nodes are running

### 2. Terminate Node3 (Leader)

On node3, press `Ctrl+C` or kill the process:
```bash
# On node3
pkill -f lms-service
# Or just Ctrl+C if running in terminal
```

### 3. Watch Node1 and Node2 Logs

You should see on node1 or node2:
```
[WARN] raft: heartbeat timeout reached, starting election
[INFO] raft: entering candidate state
[INFO] raft: election won
[INFO] raft: entering leader state
Node node1 is now the leader  (or node2)
```

### 4. Verify New Leader

Check which node became leader:
```bash
# On any remaining node, check health
curl http://159.69.23.29:8080/health
curl http://159.69.23.30:8080/health
```

One of them will show `"is_leader": true`

### 5. Test Sending Messages

Now you can send messages from the NEW leader:
```bash
# On the new leader node, type messages
test message from new leader
```

## Expected Behavior

✅ **Automatic**: No manual intervention needed
✅ **Fast**: Usually happens within 1-2 seconds
✅ **Seamless**: Cluster continues operating
✅ **Majority Required**: Needs 2 out of 3 nodes (already have this)

## What Happens

1. **Node3 dies** → Stops sending heartbeats
2. **Node1 & Node2** → Detect heartbeat timeout (usually 1-2 seconds)
3. **Election starts** → One node becomes candidate
4. **Votes exchanged** → Node1 and Node2 vote
5. **New leader** → One wins election (usually the one with most up-to-date log)
6. **Cluster continues** → New leader takes over

## Recovery

If you restart node3 later:
- It will join as a follower
- It will catch up on missed logs
- It won't automatically become leader again (unless current leader fails)

## Testing Right Now

You can test this:
1. Kill node3 (Ctrl+C)
2. Watch node1 and node2 logs
3. One will become leader within seconds
4. Send a message from the new leader
5. Restart node3 - it will join as follower

