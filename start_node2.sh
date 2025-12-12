#!/bin/bash
# Start node2 (joins existing cluster)

echo "Starting node2 (joining cluster)..."
./lms-service -id node2 -addr 159.69.23.30:7000

