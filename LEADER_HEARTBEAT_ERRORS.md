# Leader Heartbeat Errors - Normal Behavior

## What You're Seeing

When you terminate node3, node2 (the leader) shows these messages:

```
[ERROR] raft: failed to heartbeat to: peer=159.69.23.31:7000 backoff time=250ms error="dial tcp 159.69.23.31:7000: connect: connection refused"
[DEBUG] raft: failed to contact: server-id=node3 time=44.167806704s
```

## Why This Happens

### ✅ **This is NORMAL and EXPECTED behavior!**

1. **Leader's Job**: The leader (node2) sends heartbeats to ALL followers (node1, node3)
2. **Node3 is Down**: When you terminate node3, it's unreachable
3. **Connection Refused**: Leader tries to connect, gets "connection refused"
4. **Keeps Trying**: Raft keeps retrying (with backoff) in case node3 comes back

## What This Means

### ✅ **Cluster Still Works!**
- Node1 and Node2 form a **quorum** (2 out of 3 = majority)
- Cluster continues operating normally
- You can still send messages, read logs, etc.

### ✅ **No Action Needed**
- These are **informational** errors, not critical failures
- Raft handles this automatically
- The cluster is healthy with 2/3 nodes

## What Happens Next

### Scenario 1: Node3 Stays Down
- Leader keeps trying to contact node3 (these errors continue)
- Cluster works fine with node1 + node2
- No impact on functionality

### Scenario 2: Node3 Comes Back
- Node3 automatically rejoins the cluster
- Errors stop appearing
- Node3 catches up on missed logs
- Cluster is back to 3/3 nodes

## Can We Remove These Errors?

**No, and we shouldn't!** Here's why:

1. **Raft Design**: Leader must try to contact all followers
2. **Automatic Recovery**: If node3 comes back, it needs to be contacted
3. **Cluster Health**: These errors help you know which nodes are down
4. **Standard Behavior**: All Raft implementations do this

## Summary

| Question | Answer |
|----------|--------|
| **Is this a problem?** | ❌ No, it's normal |
| **Does cluster still work?** | ✅ Yes, with 2/3 nodes |
| **Should I fix this?** | ❌ No, it's expected |
| **Will errors stop?** | ✅ Yes, when node3 comes back |
| **Can I ignore these?** | ✅ Yes, they're just logs |

## What to Watch For

### ✅ **Good Signs** (What you're seeing):
- Errors about node3 (the one you killed)
- Cluster still works (can send messages)
- Node1 and Node2 are healthy

### ⚠️ **Bad Signs** (Not what you're seeing):
- Errors about ALL nodes
- No leader elected
- Can't send messages
- Cluster completely down

## Your Current Situation

✅ **Everything is working correctly!**

- Node2 is leader ✅
- Node1 is follower ✅
- Node3 is down (you terminated it) ✅
- Cluster has quorum (2/3) ✅
- Errors are just Raft trying to contact node3 ✅

**You can safely ignore these error messages.** They're just Raft doing its job!

