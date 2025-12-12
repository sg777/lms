# List Command Usage

## Overview

The `list` command allows you to view all messages stored in the Raft cluster. It works on both leader and follower nodes.

## Usage

### From CLI (Command Line)

On any node, type:
```
list
```

**On Leader Node:**
- Directly queries the FSM
- Shows all messages immediately

**On Follower Node:**
- Automatically forwards request to leader via HTTP API
- Returns the same results

### Example Output

```
=== All Messages (5 total) ===
1. hi
2. hello
3. test message
4. another message
5. final message
=============================
```

## HTTP API

You can also use the HTTP API directly:

```bash
# From leader
curl http://159.69.23.31:8080/list

# From any node (will forward to leader)
curl http://159.69.23.29:8080/list
curl http://159.69.23.30:8080/list
```

### API Response Format

```json
{
  "success": true,
  "total_count": 5,
  "messages": [
    "hi",
    "hello",
    "test message",
    "another message",
    "final message"
  ],
  "log_entries": [...]
}
```

## How It Works

1. **Leader Node**: Queries FSM directly for all messages
2. **Follower Node**: 
   - Detects it's not the leader
   - Forwards HTTP request to leader
   - Returns leader's response

## Features

✅ Works on all nodes (leader and followers)
✅ Automatic forwarding from followers to leader
✅ Shows numbered list of all messages
✅ HTTP API also available
✅ Real-time: Shows current state of cluster

## Testing

1. Send some messages from leader:
   ```
   message 1
   message 2
   message 3
   ```

2. List from leader:
   ```
   list
   ```

3. List from follower (node1 or node2):
   ```
   list
   ```
   Should show the same messages!

