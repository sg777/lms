#!/bin/bash
# Start node3 as bootstrap node

echo "Starting node3 as bootstrap node..."
./lms-service -id node3 -addr 159.69.23.31:7000 -bootstrap

