# Simple Usage - Like the Working Code

## Quick Start

### Node 1 (Bootstrap - Run First!)
```bash
./lms-service -id node1 -addr 159.69.23.29:7000 -bootstrap
```

### Node 2
```bash
./lms-service -id node2 -addr 159.69.23.30:7000
```

### Node 3
```bash
./lms-service -id node3 -addr 159.69.23.31:7000
```

## How to Use

After starting, you'll see:
```
=== LMS Service CLI ===
Node node1 running. Enter commands:
  - Type a message to send to cluster
  - Type 'list' to see all logs
  - Type 'health' to check status
  - Type 'exit' to quit
```

### Send a Message

Just type a message and press Enter:
```
hello from node1
```

This message will be replicated to all nodes in the cluster!

### Check Logs

Type `list` to see how many log entries:
```
list
Total log entries: 5
```

### Check Health

Type `health` to see node status:
```
health
State: Leader, Leader: 159.69.23.29:7000
```

### Exit

Type `exit` to quit gracefully.

## What's Fixed

1. ✅ **RPC Errors Fixed**: Transport now uses hardcoded port 7000 like working code
2. ✅ **Simple Messages**: Can send simple text messages like the original
3. ✅ **CLI Interface**: Interactive command-line like the working version
4. ✅ **HTTP API Still Works**: Port 8080 still available for API calls

## Testing

1. Start all 3 nodes
2. On the leader node, type a message
3. Check other nodes - they should see the message replicated
4. All nodes will have the same log entries

## HTTP API (Still Available)

You can still use the HTTP API on port 8080:
```bash
curl http://159.69.23.29:8080/health
curl http://159.69.23.29:8080/latest-head
```

