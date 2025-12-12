# Bootstrap Error Fix

## The Problem

**Error:** `bootstrap only works on new clusters`

This happens because:
- Bootstrap can only be done **once** on a **completely fresh** cluster
- If any node has existing Raft data, bootstrap will fail
- You tried to bootstrap node3, but the cluster already exists

## Solution 1: Don't Bootstrap Node3

**Node3 should NOT use `-bootstrap` flag!**

Only **node1** should bootstrap:

```bash
# Node 1 (ONLY ONE WITH -bootstrap)
./lms-service -id node1 -addr 159.69.23.29:7000 -bootstrap

# Node 2 (NO -bootstrap)
./lms-service -id node2 -addr 159.69.23.30:7000

# Node 3 (NO -bootstrap)
./lms-service -id node3 -addr 159.69.23.31:7000
```

## Solution 2: Start Fresh (If Needed)

If you want to start completely fresh, delete all Raft data:

```bash
# On ALL nodes, delete the raft-data directories
rm -rf ./raft-data/node1
rm -rf ./raft-data/node2
rm -rf ./raft-data/node3

# Or if using old format:
rm -rf ./raft-data-node1
rm -rf ./raft-data-node2
rm -rf ./raft-data-node3
```

Then start fresh:
1. Start node1 with `-bootstrap`
2. Start node2 and node3 WITHOUT `-bootstrap`

## Correct Startup Order

```bash
# Step 1: Start node1 FIRST with bootstrap
./lms-service -id node1 -addr 159.69.23.29:7000 -bootstrap

# Step 2: Wait for node1 to become leader, then start node2
./lms-service -id node2 -addr 159.69.23.30:7000

# Step 3: Start node3
./lms-service -id node3 -addr 159.69.23.31:7000
```

## Key Rule

**Bootstrap = Create new cluster**
- Only use `-bootstrap` on the FIRST node
- Only use it when starting a completely fresh cluster
- Never use it on nodes 2, 3, etc.

