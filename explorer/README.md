# LMS Hash Chain Explorer

A comprehensive web-based explorer for browsing and managing LMS hash chains stored in the Raft cluster, with integrated wallet management and blockchain support.

## Features

- **Browse Recent Commits**: View the latest hash chain commits in real-time
- **Search**: Search by key ID, hash, or index
- **Chain Visualization**: View complete hash chains with integrity verification
- **User Authentication**: Secure login/registration system
- **Key Management**: Generate, import, export, and delete LMS keys
- **CHIPS Wallet**: Create and manage CHIPS wallets for blockchain transactions
- **Blockchain Integration**: Per-key toggle for blockchain commits
- **Statistics Dashboard**: Monitor chain health and activity

## Quick Start

### 1. Build the Explorer

**Option A: Use the main build script (recommended)**
```bash
cd /root/lms
./build.sh
```

**Option B: Build manually**
```bash
cd /root/lms
go build -o lms-explorer ./cmd/explorer
```

### 2. Start the Explorer

**With logging to file (recommended for debugging):**
```bash
./lms-explorer -port 8081 -log-file explorer.log
```

**With default settings:**
```bash
./lms-explorer -port 8081
```

**View logs in real-time:**
```bash
tail -f explorer.log
```

### 3. Access the Web Interface

Open your browser:
- **Local**: `http://localhost:8081`
- **Remote**: `http://<server-ip>:8081`

## Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-port` | Web server port | `8081` |
| `-raft-endpoints` | Comma-separated Raft API endpoints | `http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080` |
| `-hsm-endpoint` | HSM server endpoint | `http://159.69.23.31:9090` |
| `-log-file` | Optional log file path | (stdout/stderr) |

### Examples

**Custom port:**
```bash
./lms-explorer -port 9000
```

**Custom Raft endpoints:**
```bash
./lms-explorer -raft-endpoints "http://10.0.0.1:8080,http://10.0.0.2:8080"
```

**With log file:**
```bash
./lms-explorer -port 8081 -log-file /var/log/lms-explorer.log
```

## Web Interface Guide

### Public Explorer Tab

**View Recent Commits:**
- Automatically displays the latest 50 commits
- Shows: Key ID, Index, Hash, Previous Hash
- Click any row to view the full chain
- Auto-refreshes every 10 seconds

**Search:**
- **By Key ID**: Type the key ID (e.g., `user_xxx_key_1`)
- **By Hash**: Paste a hash value
- Results show the matching entry with chain navigation

**View Blockchain Commits:**
- Click "View Blockchain" button
- Shows all commits stored on Verus blockchain
- Displays: Key ID, Label, LMS Index, Block Height, Transaction ID

### My Keys Tab (Requires Login)

**Key Management:**
- **Generate Key**: Create a new LMS key
- **Import Key**: Import an existing key
- **Export Key**: Download key for backup
- **Delete Key**: Remove a key (with confirmation)

**Wallet Balance:**
- Total wallet balance displayed next to "Import Key" button
- Shows balance across all your CHIPS wallets

**Blockchain Toggle:**
- Per-key toggle switch in the "Blockchain" column
- **Enabling**: Checks wallet balance, commits latest Raft index to blockchain
- **Future commits**: Automatically go to both Raft and blockchain
- **Disabling**: Commits only go to Raft

**Signing:**
- Select a key and message
- Click "Sign Message"
- System checks wallet balance before signing
- If blockchain is enabled, commit goes to both Raft and blockchain

### Wallet Tab (Requires Login)

**Create Wallet:**
- Click "Create New Wallet"
- System generates a new CHIPS address
- Address is stored and linked to your account

**View Wallets:**
- Lists all your CHIPS addresses
- Shows balance for each address
- Displays creation date

**Refresh Balance:**
- Click refresh button next to any address
- Fetches latest balance from CHIPS node
- Updates displayed balance

**Bulk Refresh:**
- Click "Refresh Balances" at the top
- Refreshes all wallet balances at once

## API Endpoints

### Public Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/recent?limit=N` | GET | Get recent commits (default: 50, max: 200) |
| `/api/search?q=<query>` | GET | Search by key_id, hash, or index |
| `/api/stats` | GET | Get overall statistics |
| `/api/chain/<key_id>` | GET | Get full chain for a key_id |
| `/api/blockchain` | GET | Get all blockchain commits |

### Authentication Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/auth/register` | POST | Register new user |
| `/api/auth/login` | POST | Login user |
| `/api/auth/me` | GET | Get current user info |

### Authenticated Endpoints (Require JWT Token)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/my/keys` | GET | List user's keys |
| `/api/my/generate` | POST | Generate new key |
| `/api/my/import` | POST | Import key |
| `/api/my/export` | GET | Export key |
| `/api/my/delete` | POST | Delete key |
| `/api/my/sign` | POST | Sign message |
| `/api/my/verify` | POST | Verify signature |

### Wallet Endpoints (Require JWT Token)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/my/wallet/list` | GET | List user's wallets |
| `/api/my/wallet/create` | POST | Create new wallet |
| `/api/my/wallet/balance?address=<addr>` | GET | Get balance for address |
| `/api/my/wallet/total-balance` | GET | Get total balance across all wallets |

### Blockchain Endpoints (Require JWT Token)

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/my/key/blockchain/toggle` | POST | Enable/disable blockchain for a key |
| `/api/my/key/blockchain/status` | GET | Get blockchain status for all keys |

## Example API Calls

```bash
# Get recent commits
curl http://localhost:8081/api/recent?limit=20

# Search for a key
curl http://localhost:8081/api/search?q=user_xxx_key_1

# Get statistics
curl http://localhost:8081/api/stats

# Get chain for a key
curl http://localhost:8081/api/chain/user_xxx_key_1

# Login (get token)
TOKEN=$(curl -s -X POST http://localhost:8081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user","password":"pass"}' | jq -r '.token')

# List wallets (authenticated)
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8081/api/my/wallet/list

# Get total balance
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8081/api/my/wallet/total-balance
```

## Architecture

- **Backend**: Go HTTP server (`explorer/` package)
- **Frontend**: Vanilla HTML/CSS/JavaScript (no framework dependencies)
- **Database**: BoltDB for user data, wallets, and settings
- **Data Source**: Connects to Raft cluster via HTTP API
- **Blockchain**: Verus/CHIPS RPC integration
- **Caching**: In-memory cache for recent commits (5 second TTL)
- **Auto-refresh**: Frontend polls for updates every 10 seconds

## File Structure

```
explorer/
├── server.go              # Main HTTP server and routing
├── handlers.go            # API request handlers
├── data.go                # Data fetching, caching, and processing
├── auth.go                # Authentication and JWT handling
├── wallet.go              # CHIPS wallet management
├── wallet_balance.go      # Wallet balance API
├── walletdb.go            # Wallet database (BoltDB)
├── blockchain.go          # Verus blockchain integration
├── blockchain_config.go   # Verus RPC configuration
├── key_blockchain.go      # Per-key blockchain toggle
├── key_blockchain_db.go   # Blockchain settings database
├── hsm_proxy.go           # HSM server proxy with auth
├── templates/
│   └── index.html         # Main HTML page (single-page app)
├── static/
│   ├── style.css          # Stylesheet
│   ├── app.js             # Main frontend logic
│   ├── auth.js            # Authentication UI
│   ├── mykeys.js          # My Keys tab logic
│   └── wallet.js          # Wallet tab logic
└── README.md              # This file

cmd/explorer/
└── main.go                # Entry point (parses flags, starts server)
```

## Troubleshooting

### Explorer won't start

**Error: "port already in use"**
- Solution: Use a different port with `-port 9000`
- Or kill the existing process: `pkill -f "lms-explorer"`

**Error: "connection refused" to Raft endpoints**
- Check that your Raft cluster nodes are running
- Verify the endpoint URLs are correct
- Ensure the Raft API ports (default 8080) are accessible

### No logs appearing

**If running without log file:**
- Logs go to stdout/stderr (terminal/tmux session)
- Check the terminal where you started the explorer

**If running with log file:**
- Check the log file: `tail -f explorer.log`
- Ensure the file is writable

### No data showing

**Empty recent commits table:**
- Check that you have committed some key indices
- Verify the Raft endpoints are correct
- Check the browser console for errors (F12)

**Search returns "not found":**
- Verify the key_id exists (check recent commits table)
- For hash searches, ensure you're using the exact hash value
- Check that the Raft cluster is responding

### Wallet balance shows 0 or error

**Balance not updating:**
- Ensure CHIPS node is running with `-addressindex=1`
- Check Verus RPC credentials in `explorer/blockchain_config.go`
- Verify the address exists in your CHIPS wallet
- Check explorer logs for RPC errors

**"Error refreshing balance: HTTP 500":**
- Check explorer logs for detailed error
- Verify CHIPS node is accessible
- Ensure RPC credentials are correct

### Blockchain toggle not working

**"Insufficient balance" error:**
- Fund your CHIPS wallet address
- Minimum balance needed for transaction fees (~0.001 CHIPS)
- Use the refresh button to update balance

**Toggle enabled but commits not going to blockchain:**
- Check explorer logs for blockchain commit errors
- Verify Verus identity is configured correctly
- Ensure CHIPS node is synced and responding

### Browser can't connect

**"Connection refused" or "This site can't be reached":**
- If running on a remote server, use the server's IP: `http://<server-ip>:8081`
- Check firewall settings on port 8081
- Ensure the explorer process is running: `ps aux | grep lms-explorer`

## Integration with Your Cluster

The explorer connects to:
- **Raft Cluster**: HTTP API for hash chain data
- **HSM Server**: For key operations and signing
- **CHIPS Node**: Verus RPC for blockchain operations

The explorer will try each Raft endpoint until one responds successfully, providing automatic failover if a node is down.

## Security Notes

- All authenticated endpoints require a valid JWT token
- Tokens expire after 24 hours
- Wallet operations are user-scoped (users can only access their own wallets)
- Keys are stored securely in the HSM server
- Blockchain transactions use explicit funding addresses from user wallets

## Next Steps

Once the explorer is running:
1. Register a user account
2. Create a CHIPS wallet
3. Generate or import an LMS key
4. Enable blockchain for the key (optional)
5. Start signing messages and viewing commits

For more information, see the main project documentation in `/root/lms/docs/`.
