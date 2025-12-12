#!/bin/bash

echo "🔧 Rebuild script for all Raft nodes"
echo ""
echo "IMPORTANT: You must rebuild lms-service on ALL Raft nodes!"
echo ""
echo "On each Raft node (159.69.23.29, 159.69.23.30, 159.69.23.31), run:"
echo ""
echo "  cd /root/lms"
echo "  git pull"
echo "  go build -o lms-service ./main.go"
echo "  # Then restart your lms-service"
echo ""
echo "The signature verification code has been updated."
echo "Old code will reject valid signatures!"

