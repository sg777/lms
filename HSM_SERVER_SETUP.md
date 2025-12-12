# HSM Server Setup Guide

## Starting HSM Server

The HSM server needs to be started with the Raft cluster endpoints. Use the actual IPs of your Raft nodes:

```bash
# On node 159.69.23.29 (or any node)
./hsm-server -port 9090 -raft-endpoints "http://159.69.23.29:8080,http://159.69.23.30:8080,http://159.69.23.31:8080"
```

## Using HSM Client

Always specify the HSM server IP (not localhost) when using the client:

```bash
# Sign a message
./hsm-client sign -key-id my_key -msg "hello" -server http://159.69.23.29:9090

# Query Raft cluster
./hsm-client query -key-id my_key -raft http://159.69.23.29:8080

# Generate key
./hsm-client generate -key-id my_key -server http://159.69.23.29:9090

# List keys
./hsm-client list -server http://159.69.23.29:9090
```

## Important Notes

1. **HSM Server IP**: Always use the actual IP of the node where HSM server is running
2. **Raft Endpoints**: HSM server needs all Raft node IPs to query/commit
3. **Default localhost**: The defaults use localhost, so you must specify IPs when running on different machines

