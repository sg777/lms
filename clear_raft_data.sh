#!/bin/bash
# Clear all Raft data for fresh start

echo "Clearing all Raft data..."

# Remove all raft data directories
rm -rf ./raft-data/node1
rm -rf ./raft-data/node2
rm -rf ./raft-data/node3
rm -rf ./raft-data-node1
rm -rf ./raft-data-node2
rm -rf ./raft-data-node3

echo "✅ All Raft data cleared!"
echo ""
echo "You can now start fresh:"
echo "  ./start_node3_bootstrap.sh"

