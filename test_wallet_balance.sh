#!/bin/bash
# Test script to check wallet balance endpoint

TOKEN="YOUR_TOKEN_HERE"  # Replace with actual token from browser localStorage
ADDRESS="RNdtBgwRvPTvp2ooMZNrV75PEa9s4UvEV9"
URL="http://159.69.23.31:8081/api/my/wallet/balance?address=${ADDRESS}"

echo "Testing wallet balance endpoint..."
echo "URL: $URL"
echo ""

curl -v -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     "$URL"

echo ""
echo "Done."

