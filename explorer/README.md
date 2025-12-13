# LMS Hash Chain Explorer

A standalone web-based explorer for browsing and searching the LMS hash chain stored in the Raft cluster.

## Quick Start

### 1. Build the Explorer

```bash
cd /root/lms
go build -o lms-explorer ./cmd/explorer
```

This will create the `lms-explorer` binary in the current directory.

### 2. Start the Explorer

The explorer needs to know where your Raft cluster nodes are running. Specify the API endpoints:

```bash
./lms-explorer -port 8081 -raft-endpoints "http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080"
```

**Or use defaults** (if your cluster uses the default IPs):
```bash
./lms-explorer
```

The defaults are: `http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080`

### 3. Access the Web Interface

Open your web browser and navigate to:

**If running locally:**
```
http://localhost:8081
```

**If running on a remote server (replace with your server IP):**
```
http://159.69.23.29:8081
```

You should see the LMS Hash Chain Explorer interface.

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-port` | Port number for the explorer web server | `8081` |
| `-raft-endpoints` | Comma-separated list of Raft API endpoints | `http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080` |

### Examples

**Custom port:**
```bash
./explorer -port 9000
```

**Custom Raft endpoints:**
```bash
./explorer -raft-endpoints "http://10.0.0.1:8080,http://10.0.0.2:8080"
```

**Both custom:**
```bash
./explorer -port 9000 -raft-endpoints "http://10.0.0.1:8080,http://10.0.0.2:8080"
```

## How to Use the Web Interface

### View Recent Commits

1. When you open the explorer, recent commits are displayed automatically in a table
2. The table shows:
   - **Key ID**: The identifier for the LMS key
   - **Index**: The LMS index used
   - **Hash**: The hash of this commit (truncated for display)
   - **Previous Hash**: The hash of the previous commit (truncated)
3. **Click on any row** to view the full chain for that key_id
4. The list auto-refreshes every 10 seconds

### Search for a Key

**Search by Key ID:**
1. Type a key_id in the search bar (e.g., `lms_key_1`)
2. Click "Search" or press Enter
3. The full chain for that key will be displayed
4. You'll see all entries in the chain, with hash links between them

**Example:**
- Type: `lms_key_1`
- Result: Shows the complete hash chain for `lms_key_1` with all entries

### Search by Hash

1. Copy a hash value (e.g., from the recent commits table or chain view)
2. Paste it into the search bar
3. Click "Search"
4. The explorer will find which entry uses that hash

**Example:**
- Type: `g5WYnCjbDHzQdzvb7uOkmueXZHxbt5VOIO5oLCAi6zQ=`
- Result: Shows the entry that has this hash, with a link to view the full chain

### View Statistics

The statistics dashboard at the top shows:
- **Total Keys**: Number of unique key_ids in the cluster
- **Total Commits**: Total number of index commitments
- **Valid Chains**: Number of chains that pass integrity checks
- **Broken Chains**: Number of chains with hash chain breaks
- **Last Updated**: Timestamp of the most recent commit

Statistics update automatically every 10 seconds.

### View Full Chain Details

When viewing a chain, you'll see:

1. **Chain Status Badge**: 
   - ✓ VALID (green) = Chain integrity is verified
   - ✗ BROKEN (red) = Chain has integrity issues

2. **Chain Entries**:
   - Each entry shows:
     - Entry number and total count (e.g., "Entry 1 of 5")
     - LMS Index
     - Key ID
     - Previous Hash (links to previous entry)
     - Current Hash (links to next entry)
     - Signature
     - Verification status

3. **Visual Links**: Arrows (↓) between entries show the chain structure

4. **Genesis Entry**: The first entry is highlighted in blue, showing it links to the genesis hash

## Example Workflow

### Scenario: You want to inspect a specific key's history

1. **Start the explorer:**
   ```bash
   ./lms-explorer -port 8081
   ```

2. **Open browser:**
   ```
   http://localhost:8081
   ```

3. **Find the key in recent commits table:**
   - Look at the "Recent Commits" table
   - Find the row with your key_id (e.g., `lms_key_1`)
   - Click on that row

4. **View the chain:**
   - The chain view opens below
   - You see all entries for that key
   - Green borders = valid entries
   - Check the "Chain Status" badge at the top

5. **Inspect a specific entry:**
   - Scroll through the entries
   - Each entry shows its hash, previous hash, and signature
   - Hover over hashes to see full values (if truncated)

### Scenario: You have a hash and want to find its entry

1. **Start the explorer** (if not running)

2. **Open browser and navigate to the explorer**

3. **Paste the hash into the search bar:**
   - Example: `g5WYnCjbDHzQdzvb7uOkmueXZHxbt5VOIO5oLCAi6zQ=`

4. **Click "Search"**

5. **View the result:**
   - The entry matching that hash is displayed
   - Click "View Full Chain" to see the entire chain for that key

## Troubleshooting

### Explorer won't start

**Error: "port already in use"**
- Solution: Use a different port with `-port 9000`

**Error: "connection refused" to Raft endpoints**
- Check that your Raft cluster nodes are running
- Verify the endpoint URLs are correct
- Ensure the Raft API ports (default 8080) are accessible

### No data showing

**Empty recent commits table:**
- Check that you have committed some key indices using `hsm-client sign`
- Verify the Raft endpoints are correct
- Check the browser console for errors (F12)

**Search returns "not found":**
- Verify the key_id exists (check recent commits table)
- For hash searches, ensure you're using the exact hash value
- Check that the Raft cluster is responding

### Browser can't connect

**"Connection refused" or "This site can't be reached":**
- If running on a remote server, use the server's IP address: `http://<server-ip>:8081`
- Check firewall settings on port 8081
- Ensure the explorer process is running (`ps aux | grep explorer`)

## API Endpoints (for developers)

The explorer exposes REST API endpoints:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/recent?limit=N` | GET | Get recent commits (default: 50, max: 200) |
| `/api/search?q=<query>` | GET | Search by key_id, hash, or index |
| `/api/stats` | GET | Get overall statistics |
| `/api/chain/<key_id>` | GET | Get full chain for a key_id |

**Example API calls:**

```bash
# Get recent 20 commits
curl http://localhost:8081/api/recent?limit=20

# Search for a key
curl http://localhost:8081/api/search?q=lms_key_1

# Get statistics
curl http://localhost:8081/api/stats

# Get chain for a key
curl http://localhost:8081/api/chain/lms_key_1
```

## Architecture

- **Backend**: Go HTTP server (`explorer/` package)
- **Frontend**: Vanilla HTML/CSS/JavaScript (no framework dependencies)
- **Data Source**: Connects to Raft cluster via HTTP API
- **Caching**: In-memory cache for recent commits (5 second TTL)
- **Auto-refresh**: Frontend polls for updates every 10 seconds

## File Structure

```
explorer/
├── server.go          # Main HTTP server and routing
├── handlers.go        # API request handlers
├── data.go           # Data fetching, caching, and processing
├── templates/
│   └── index.html    # Main HTML page (single-page app)
├── static/
│   ├── style.css     # Stylesheet (modern, responsive)
│   └── app.js        # Frontend JavaScript (API calls, UI updates)
└── README.md         # This file

cmd/explorer/
└── main.go           # Entry point (parses flags, starts server)
```

## Integration with Your Cluster

The explorer connects to your Raft cluster using the same HTTP API that `hsm-client` uses:
- `/list` - Get all log entries (used to discover keys)
- `/key/<key_id>/chain` - Get full chain for a key_id
- `/key/<key_id>/index` - Get last index for a key_id

The explorer will try each endpoint in the provided list until one responds successfully. This provides automatic failover if a node is down.

## Next Steps

Once the explorer is running, you can:
1. Browse recent commits to see what's happening
2. Search for specific keys or hashes
3. Inspect chain integrity
4. Monitor statistics to track chain health

For more information about the LMS system, see the main project documentation.
