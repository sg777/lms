# LMS Hash Chain Explorer

A standalone web-based explorer for browsing and searching the LMS hash chain stored in the Raft cluster.

## Features

- **Search Bar**: Search by `key_id`, `hash`, or `index`
- **Recent Commits**: Display the last 50 commits with key_id, index, and hashes
- **Statistics Dashboard**: 
  - Total keys
  - Total commits
  - Valid chains count
  - Broken chains count
  - Last commit timestamp
- **Chain Visualization**: Full hash chain view with:
  - Visual links between entries
  - Hash chain integrity verification
  - Genesis entry highlighting
  - Broken chain detection
- **Auto-refresh**: Automatically updates every 10 seconds

## Usage

### Start the Explorer

```bash
./explorer -port 8081 -raft-endpoints "http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080"
```

Or use defaults:
```bash
./explorer
```

### Access the Web Interface

Open your browser and navigate to:
```
http://localhost:8081
```

Or if running on a remote server:
```
http://<server-ip>:8081
```

## Command Line Options

- `-port`: Port to run the explorer server on (default: 8081)
- `-raft-endpoints`: Comma-separated list of Raft cluster API endpoints (default: configured cluster IPs)

## API Endpoints

The explorer exposes the following API endpoints:

- `GET /api/recent?limit=N` - Get recent commits (default limit: 50, max: 200)
- `GET /api/search?q=<query>` - Search by key_id, hash, or index
- `GET /api/stats` - Get overall statistics
- `GET /api/chain/<key_id>` - Get full chain for a key_id

## Architecture

- **Backend**: Go HTTP server (`explorer/` package)
- **Frontend**: Vanilla HTML/CSS/JavaScript (no framework dependencies)
- **Data Source**: Connects to Raft cluster via HTTP API
- **Caching**: In-memory cache for recent commits (5 second TTL)

## File Structure

```
explorer/
├── server.go          # Main HTTP server
├── handlers.go        # API request handlers
├── data.go           # Data fetching and processing logic
├── templates/
│   └── index.html    # Main HTML page
├── static/
│   ├── style.css     # Stylesheet
│   └── app.js        # Frontend JavaScript
└── README.md         # This file

cmd/explorer/
└── main.go           # Entry point
```

## Example Workflows

### View Recent Commits
1. Open the explorer in your browser
2. Recent commits are displayed automatically in the table
3. Click on any row to view the full chain for that key_id

### Search for a Key
1. Enter a `key_id` (e.g., `lms_key_1`) in the search bar
2. Click "Search" or press Enter
3. View the full chain with all entries

### Search by Hash
1. Enter a hash value in the search bar (e.g., `g5WYnCjbDHzQdzvb7uOkmueXZHxbt5VOIO5oLCAi6zQ=`)
2. Click "Search"
3. View the entry that matches that hash

### View Statistics
1. Statistics are displayed at the top of the page
2. Updates automatically every 10 seconds
3. Shows overall health of the hash chains

## Integration with Raft Cluster

The explorer connects to the Raft cluster using the same HTTP API endpoints used by the HSM client:
- `/list` - Get all log entries
- `/key/<key_id>/chain` - Get full chain for a key_id
- `/key/<key_id>/index` - Get last index for a key_id

The explorer will try each endpoint in the provided list until one responds successfully.

