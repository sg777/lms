# Node3 as Bootstrap Node

## Setup Complete ✅

All Raft data has been cleared. Node3 (159.69.23.31) is now configured as the bootstrap node.

## Startup Order

### Step 1: Start Node3 FIRST (Bootstrap)
```bash
./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
```

Or use the script:
```bash
./start_node3_bootstrap.sh
```

**Wait for:** "Node node3 is now the leader"

### Step 2: Start Node1 (Joins Cluster)
```bash
./lms-service -id node1 -addr 159.69.23.29:7000
```

Or use the script:
```bash
./start_node1.sh
```

### Step 3: Start Node2 (Joins Cluster)
```bash
./lms-service -id node2 -addr 159.69.23.30:7000
```

Or use the script:
```bash
./start_node2.sh
```

## Important Notes

- ✅ Node3 is the bootstrap node (creates the cluster)
- ✅ Node1 and Node2 join the existing cluster (NO -bootstrap flag)
- ✅ All existing Raft data has been cleared
- ✅ Start node3 FIRST, wait for it to become leader, then start others

## Verification

After all nodes are running, check the cluster:

```bash
# Check health on any node
curl http://159.69.23.31:8080/health

# Should show node3 as leader
curl http://159.69.23.31:8080/leader
```

## If You Need to Start Fresh Again

```bash
# Delete all Raft data
rm -rf ./raft-data/node1 ./raft-data/node2 ./raft-data/node3

# Then start node3 with bootstrap again
./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap
```

