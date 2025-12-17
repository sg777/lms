#!/bin/bash
# Clear all Raft data for fresh start

echo "Clearing all Raft data..."

# Remove all raft data directories (standard pattern: ./raft-data/nodeX)
# This removes: raft.db, snapshots/, and all other Raft files
rm -rf ./raft-data/node1
rm -rf ./raft-data/node2
rm -rf ./raft-data/node3

# Remove alternative pattern directories (./raft-data-nodeX) if they exist
rm -rf ./raft-data-node1
rm -rf ./raft-data-node2
rm -rf ./raft-data-node3

# Remove parent raft-data directory if it's empty (cleanup)
if [ -d "./raft-data" ] && [ -z "$(ls -A ./raft-data 2>/dev/null)" ]; then
    rmdir ./raft-data 2>/dev/null || true
fi

echo "✅ All Raft data cleared!"
echo ""
echo "Removed:"
echo "  - ./raft-data/node1 (raft.db, snapshots, etc.)"
echo "  - ./raft-data/node2 (raft.db, snapshots, etc.)"
echo "  - ./raft-data/node3 (raft.db, snapshots, etc.)"
echo "  - ./raft-data-node1, ./raft-data-node2, ./raft-data-node3 (if existed)"
echo ""
echo "You can now start fresh:"
echo "  ./start_node3_bootstrap.sh"

