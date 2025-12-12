#!/bin/bash
# Diagnostic script for leader failover issues

echo "=== Diagnosing Leader Failover Issue ==="
echo ""

echo "1. Checking if nodes are running..."
echo "Node1 (159.69.23.29:8080):"
curl -s http://159.69.23.29:8080/health 2>/dev/null | head -3 || echo "  ❌ Node1 not responding"
echo ""

echo "Node2 (159.69.23.30:8080):"
curl -s http://159.69.23.30:8080/health 2>/dev/null | head -3 || echo "  ❌ Node2 not responding"
echo ""

echo "Node3 (159.69.23.31:8080):"
curl -s http://159.69.23.31:8080/health 2>/dev/null | head -3 || echo "  ❌ Node3 not responding"
echo ""

echo "2. Checking current leaders..."
echo "Node1 leader info:"
curl -s http://159.69.23.29:8080/leader 2>/dev/null || echo "  ❌ Cannot get leader info"
echo ""

echo "Node2 leader info:"
curl -s http://159.69.23.30:8080/leader 2>/dev/null || echo "  ❌ Cannot get leader info"
echo ""

echo "3. Network connectivity test (if you're on one of the nodes):"
echo "  Test from node1 to node2: telnet 159.69.23.30 7000"
echo "  Test from node2 to node1: telnet 159.69.23.29 7000"
echo ""

echo "=== Diagnosis Complete ==="

