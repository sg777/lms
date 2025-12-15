#!/bin/bash
# Script to restart explorer with logging

echo "Stopping existing explorer process..."
pkill -f "lms-explorer" || echo "No existing process found"

sleep 2

echo "Starting explorer with log file..."
cd /root/lms
./lms-explorer -port 8081 -log-file explorer.log &
EXPLORER_PID=$!

echo "Explorer started with PID: $EXPLORER_PID"
echo "Logs are being written to: /root/lms/explorer.log"
echo ""
echo "To view logs in real-time, run:"
echo "  tail -f /root/lms/explorer.log"
echo ""
echo "Or to see the last 50 lines:"
echo "  tail -n 50 /root/lms/explorer.log"

