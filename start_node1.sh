#!/bin/bash
# Start node1 (joins existing cluster)

echo "Starting node1 (joining cluster)..."
./lms-service -id node1 -addr 159.69.23.29:7000

