# Bootstrap Explained - Option 3

## How It Works

### Bootstrap (One-Time Setup)

**Bootstrap is ONLY needed ONCE** when creating a brand new cluster. It tells Raft:
- "These are the initial nodes: node1, node2, node3"
- Stores this configuration in Raft's persistent log

### After Bootstrap

Once the cluster is bootstrapped:
- ✅ **Nodes can start in ANY order**
- ✅ **They read cluster config from disk** (from previous bootstrap)
- ✅ **They automatically join the existing cluster**
- ✅ **Leader election happens automatically** when 2+ nodes are available

## Current Workflow

### First Time (Fresh Cluster)

1. **Start node3 with bootstrap** (creates the cluster):
   ```bash
   ./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
   ```

2. **Start node1** (joins existing cluster):
   ```bash
   ./lms-service -id node1 -addr 159.69.23.29:7000
   ```

3. **Start node2** (joins existing cluster):
   ```bash
   ./lms-service -id node2 -addr 159.69.23.30:7000
   ```

### Subsequent Starts (Cluster Already Exists)

After the first bootstrap, you can start nodes in **ANY order**:

```bash
# Start in any order - they all read the cluster config from disk
./lms-service -id node2 -addr 159.69.23.30:7000
./lms-service -id node1 -addr 159.69.23.29:7000
./lms-service -id node3 -addr 159.69.23.31:7000
```

**No bootstrap needed!** They automatically join because the cluster config is stored on disk.

## When to Bootstrap Again

Only if you want to **completely reset** the cluster:

1. Delete all Raft data:
   ```bash
   ./clear_raft_data.sh
   ```

2. Bootstrap again (first node only):
   ```bash
   ./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
   ```

## Why This Works

- **Bootstrap** = Create initial cluster configuration
- **Persistent Storage** = Cluster config saved to disk (raft.db)
- **Auto-Join** = Nodes read config from disk and join automatically
- **Leader Election** = Happens automatically when 2+ nodes are available

## Summary

- ✅ Bootstrap **once** to create cluster
- ✅ After that, start nodes in **any order**
- ✅ They automatically join and elect leader
- ✅ No manual bootstrap needed for subsequent starts

This is the standard Raft approach and works perfectly for your use case!

