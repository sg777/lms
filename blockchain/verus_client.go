package blockchain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// VerusClient provides an interface to interact with Verus/CHIPS blockchain via RPC
type VerusClient struct {
	rpcURL      string
	rpcUser     string
	rpcPassword string
	httpClient  *http.Client
}

// NewVerusClient creates a new Verus RPC client
// rpcURL: e.g., "http://127.0.0.1:22778" for CHIPS chain
// rpcUser: RPC username
// rpcPassword: RPC password
func NewVerusClient(rpcURL, rpcUser, rpcPassword string) *VerusClient {
	return &VerusClient{
		rpcURL:      rpcURL,
		rpcUser:     rpcUser,
		rpcPassword: rpcPassword,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewVerusClientFromConfig creates a Verus client from a config file path
// configPath: Path to the Verus config file (e.g., ~/.verus/pbaas/.../config.conf)
func NewVerusClientFromConfig(configPath string) (*VerusClient, error) {
	// TODO: Parse config file to extract rpcuser, rpcpassword, rpcport, rpchost
	// For now, we'll use the hardcoded values from the actual config
	// This should be implemented to read from file
	return nil, fmt.Errorf("NewVerusClientFromConfig not yet implemented - use NewVerusClient")
}

// RPCRequest represents a JSON-RPC request
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// RPCResponse represents a JSON-RPC response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError represents an RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// callRPC makes a JSON-RPC call to the Verus node
func (v *VerusClient) callRPC(method string, params []interface{}) (json.RawMessage, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal RPC request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", v.rpcURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %v", err)
	}

	httpReq.SetBasicAuth(v.rpcUser, v.rpcPassword)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("RPC call failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rpcResp RPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, fmt.Errorf("failed to decode RPC response: %v", err)
	}

	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC error: %s (code: %d)", rpcResp.Error.Message, rpcResp.Error.Code)
	}

	return rpcResp.Result, nil
}

// GetBlockchainInfo returns blockchain information
func (v *VerusClient) GetBlockchainInfo() (map[string]interface{}, error) {
	result, err := v.callRPC("getblockchaininfo", []interface{}{})
	if err != nil {
		return nil, err
	}

	var info map[string]interface{}
	if err := json.Unmarshal(result, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blockchain info: %v", err)
	}

	return info, nil
}

// GetBestBlockHash returns the hash of the best block
func (v *VerusClient) GetBestBlockHash() (string, error) {
	result, err := v.callRPC("getbestblockhash", []interface{}{})
	if err != nil {
		return "", err
	}

	var hash string
	if err := json.Unmarshal(result, &hash); err != nil {
		return "", fmt.Errorf("failed to unmarshal block hash: %v", err)
	}

	return hash, nil
}

// VDXFIDResponse represents the response from getvdxfid
type VDXFIDResponse struct {
	VDXFID        string `json:"vdxfid"`         // Normalized i-ID (base58check)
	IndexID       string `json:"indexid"`        // Index ID
	Hash160Result string `json:"hash160result"`  // 20-byte hash in hex
	QualifiedName struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		ParentID  string `json:"parentid,omitempty"`
	} `json:"qualifiedname"`
}

// GetVDXFID computes the normalized VDXF ID (normalized key) for a given string
// This is what Verus uses internally to normalize key names in contentmultimap
func (v *VerusClient) GetVDXFID(keyName string) (string, error) {
	result, err := v.callRPC("getvdxfid", []interface{}{keyName})
	if err != nil {
		return "", err
	}

	var vdxfIDResp VDXFIDResponse
	if err := json.Unmarshal(result, &vdxfIDResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal VDXF ID response: %v", err)
	}

	return vdxfIDResp.VDXFID, nil
}

// GetBlockHeight returns the current block height
func (v *VerusClient) GetBlockHeight() (int64, error) {
	info, err := v.GetBlockchainInfo()
	if err != nil {
		return 0, err
	}

	blocks, ok := info["blocks"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid blocks value in blockchain info")
	}

	return int64(blocks), nil
}

// Identity represents a Verus identity
type Identity struct {
	Name              string                 `json:"name"`
	Parent            string                 `json:"parent"`
	SystemID          string                 `json:"systemid,omitempty"`
	ContentMap        map[string]interface{} `json:"contentmap,omitempty"`
	ContentMultiMap   map[string]interface{} `json:"contentmultimap,omitempty"`
	PrimaryAddresses  []string               `json:"primaryaddresses,omitempty"`
	MinimumSignatures int                    `json:"minimumsignatures,omitempty"`
	RevocationAuth    string                 `json:"revocationauthority,omitempty"`
	RecoveryAuth      string                 `json:"recoveryauthority,omitempty"`
	TimeLock          int64                  `json:"timelock,omitempty"`
	Version           int                    `json:"version,omitempty"`
	Flags             int                    `json:"flags,omitempty"`
}

// GetIdentityResponse represents the full identity response from Verus
type GetIdentityResponse struct {
	FriendlyName        string   `json:"friendlyname"`
	FullyQualifiedName  string   `json:"fullyqualifiedname"`
	Identity            Identity `json:"identity"`
	Status              string   `json:"status"`
	CanSpendFor         bool     `json:"canspendfor"`
	CanSignFor          bool     `json:"cansignfor"`
	BlockHeight         int64    `json:"blockheight"`
	TxID                string   `json:"txid"`
	Vout                int      `json:"vout"`
}

// AttestationCommit represents an LMS attestation committed to blockchain via identity
type AttestationCommit struct {
	KeyID          string    `json:"key_id"`          // LMS key ID
	LMSIndex       string    `json:"lms_index"`       // LMS index committed
	BlockHeight    int64     `json:"block_height"`    // Block height where committed
	TxID           string    `json:"txid"`            // Transaction ID
	Timestamp      time.Time `json:"timestamp"`       // Block timestamp
}

// GetIdentity retrieves identity information
func (v *VerusClient) GetIdentity(identityName string) (*GetIdentityResponse, error) {
	result, err := v.callRPC("getidentity", []interface{}{identityName})
	if err != nil {
		return nil, err
	}

	var identity GetIdentityResponse
	if err := json.Unmarshal(result, &identity); err != nil {
		return nil, fmt.Errorf("failed to unmarshal identity: %v", err)
	}

	return &identity, nil
}

// UpdateIdentity updates an identity's contentmultimap with LMS index commit
// identityName: e.g., "sg777z.chips.vrsc@"
// keyID: The LMS key ID
// lmsIndex: The LMS index to commit
// Returns the transaction ID
func (v *VerusClient) UpdateIdentity(identityName, keyID, lmsIndex string) (string, error) {
	// First, get current identity to preserve existing fields
	current, err := v.GetIdentity(identityName)
	if err != nil {
		return "", fmt.Errorf("failed to get current identity: %v", err)
	}

	// Prepare the identity update JSON
	identityUpdate := Identity{
		Name:    current.Identity.Name,
		Parent:  current.Identity.Parent,
		SystemID: current.Identity.SystemID,
		ContentMultiMap: make(map[string]interface{}),
	}

	// Preserve existing contentmultimap if it exists
	if current.Identity.ContentMultiMap != nil {
		for k, v := range current.Identity.ContentMultiMap {
			identityUpdate.ContentMultiMap[k] = v
		}
	}

	// Add or update the key_id entry (REPLACE latest only, not append history)
	// Format: "key_id": [{"iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c": "<lms_index>"}]
	// The "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c" is a constant identifier
	// Note: We replace the entire array with just the new index (Option B)
	// History is preserved in blockchain via getidentityhistory API
	const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"
	
	// Replace the key_id entry with just the new index (not appending to history)
	// History is available via GetIdentityHistory() method
	newEntry := map[string]string{
		mapKey: lmsIndex,
	}
	keyEntries := []map[string]string{newEntry}

	// Update the contentmultimap (replaces previous value for this key_id)
	identityUpdate.ContentMultiMap[keyID] = keyEntries

	// Convert identityUpdate to map[string]interface{} for RPC call
	// Verus RPC expects the identity as an object, not a JSON string
	identityMap := make(map[string]interface{})
	if identityUpdate.Name != "" {
		identityMap["name"] = identityUpdate.Name
	}
	if identityUpdate.Parent != "" {
		identityMap["parent"] = identityUpdate.Parent
	}
	if identityUpdate.SystemID != "" {
		identityMap["systemid"] = identityUpdate.SystemID
	}
	if len(identityUpdate.ContentMultiMap) > 0 {
		identityMap["contentmultimap"] = identityUpdate.ContentMultiMap
	}

	// Call updateidentity RPC with the identity object
	result, err := v.callRPC("updateidentity", []interface{}{identityMap})
	if err != nil {
		return "", fmt.Errorf("failed to update identity: %v", err)
	}

	var txID string
	if err := json.Unmarshal(result, &txID); err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction ID: %v", err)
	}

	return txID, nil
}

// QueryAttestationCommits queries identity's contentmultimap for LMS index commits
// identityName: e.g., "sg777z.chips.vrsc@"
// keyID: The LMS key ID to query (optional, empty string for all)
// Returns all attestation commits found in the identity
func (v *VerusClient) QueryAttestationCommits(identityName, keyID string) ([]*AttestationCommit, error) {
	identity, err := v.GetIdentity(identityName)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %v", err)
	}

	var commits []*AttestationCommit
	const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"

	if identity.Identity.ContentMultiMap == nil {
		return commits, nil
	}

	// If keyID specified, only look at that key
	keysToCheck := []string{keyID}
	if keyID == "" {
		// Check all keys
		for k := range identity.Identity.ContentMultiMap {
			keysToCheck = append(keysToCheck, k)
		}
	}

	for _, checkKey := range keysToCheck {
		if checkKey == "" {
			continue
		}

		entries, ok := identity.Identity.ContentMultiMap[checkKey].([]interface{})
		if !ok {
			continue
		}

		for _, entry := range entries {
			if entryMap, ok := entry.(map[string]interface{}); ok {
				if lmsIndex, ok := entryMap[mapKey].(string); ok {
					commits = append(commits, &AttestationCommit{
						KeyID:       checkKey,
						LMSIndex:    lmsIndex,
						BlockHeight: identity.BlockHeight,
						TxID:        identity.TxID,
						Timestamp:   time.Now(), // TODO: Get actual block timestamp
					})
				}
			}
		}
	}

	return commits, nil
}

// GetLatestIndexByKeyID returns the latest committed LMS index for each key_id
// Returns a map: keyID -> latest LMS index
func (v *VerusClient) GetLatestIndexByKeyID(identityName string) (map[string]string, error) {
	commits, err := v.QueryAttestationCommits(identityName, "")
	if err != nil {
		return nil, err
	}

	// Group by key_id and find latest by block height
	latestByKey := make(map[string]*AttestationCommit)
	for _, commit := range commits {
		if existing, ok := latestByKey[commit.KeyID]; !ok || commit.BlockHeight > existing.BlockHeight {
			latestByKey[commit.KeyID] = commit
		}
	}

	// Extract just the indices
	result := make(map[string]string)
	for keyID, commit := range latestByKey {
		result[keyID] = commit.LMSIndex
	}

	return result, nil
}

// GetLatestIndexForKey returns the latest committed LMS index for a specific key_id
func (v *VerusClient) GetLatestIndexForKey(identityName, keyID string) (string, error) {
	commits, err := v.QueryAttestationCommits(identityName, keyID)
	if err != nil {
		return "", err
	}

	if len(commits) == 0 {
		return "", fmt.Errorf("no commits found for key_id: %s", keyID)
	}

	// Find latest by block height
	latest := commits[0]
	for _, commit := range commits[1:] {
		if commit.BlockHeight > latest.BlockHeight {
			latest = commit
		}
	}

	return latest.LMSIndex, nil
}

// GetAllKeyIDs returns all key_ids that have committed indices
func (v *VerusClient) GetAllKeyIDs(identityName string) ([]string, error) {
	identity, err := v.GetIdentity(identityName)
	if err != nil {
		return nil, err
	}

	if identity.Identity.ContentMultiMap == nil {
		return []string{}, nil
	}

	var keyIDs []string
	for keyID := range identity.Identity.ContentMultiMap {
		keyIDs = append(keyIDs, keyID)
	}

	return keyIDs, nil
}

// IdentityHistoryEntry represents a single identity state in history
type IdentityHistoryEntry struct {
	Identity    Identity `json:"identity"`
	BlockHash   string   `json:"blockhash"`
	Height      int64    `json:"height"`
	Output      struct {
		TxID    string `json:"txid"`
		VoutNum int    `json:"voutnum"`
	} `json:"output"`
}

// GetIdentityHistoryResponse represents the full history response
type GetIdentityHistoryResponse struct {
	FullyQualifiedName string                 `json:"fullyqualifiedname"`
	Status             string                 `json:"status"`
	CanSpendFor        bool                   `json:"canspendfor"`
	CanSignFor         bool                   `json:"cansignfor"`
	BlockHeight        int64                  `json:"blockheight"`
	TxID               string                 `json:"txid"`
	Vout               int                    `json:"vout"`
	History            []IdentityHistoryEntry `json:"history"`
}

// GetIdentityHistory retrieves the full history of identity updates
// heightStart: Starting block height (0 = from genesis)
// heightEnd: Ending block height (0 = current height, -1 = include mempool)
func (v *VerusClient) GetIdentityHistory(identityName string, heightStart, heightEnd int64) (*GetIdentityHistoryResponse, error) {
	params := []interface{}{identityName}
	if heightStart > 0 {
		params = append(params, heightStart)
		if heightEnd > 0 {
			params = append(params, heightEnd)
		}
	} else if heightEnd > 0 {
		// If only heightEnd specified, need to provide heightStart as 0
		params = append(params, 0, heightEnd)
	}

	result, err := v.callRPC("getidentityhistory", params)
	if err != nil {
		return nil, err
	}

	var history GetIdentityHistoryResponse
	if err := json.Unmarshal(result, &history); err != nil {
		return nil, fmt.Errorf("failed to unmarshal identity history: %v", err)
	}

	return &history, nil
}

// GetLMSIndexHistory returns the history of LMS index commits for a specific key_id
// Returns commits ordered by block height (oldest first)
func (v *VerusClient) GetLMSIndexHistory(identityName, keyID string, heightStart, heightEnd int64) ([]*AttestationCommit, error) {
	history, err := v.GetIdentityHistory(identityName, heightStart, heightEnd)
	if err != nil {
		return nil, err
	}

	var commits []*AttestationCommit
	const mapKey = "iK7a5JNJnbeuYWVHCDRpJosj3irGJ5Qa8c"

	for _, entry := range history.History {
		if entry.Identity.ContentMultiMap == nil {
			continue
		}

		// Check if this entry has the key_id we're looking for
		if entries, ok := entry.Identity.ContentMultiMap[keyID].([]interface{}); ok {
			for _, item := range entries {
				if entryMap, ok := item.(map[string]interface{}); ok {
					if lmsIndex, ok := entryMap[mapKey].(string); ok {
						commits = append(commits, &AttestationCommit{
							KeyID:       keyID,
							LMSIndex:    lmsIndex,
							BlockHeight: entry.Height,
							TxID:        entry.Output.TxID,
							Timestamp:   time.Now(), // TODO: Get actual block timestamp if available
						})
						// Only take the first (latest) entry from each identity update
						break
					}
				}
			}
		}
	}

	return commits, nil
}


// CommitLMSIndexWithPubkeyHash commits an LMS index using pubkey_hash as the key
// This is the recommended way since pubkey_hash is deterministic and we can pre-compute the normalized ID
// Returns: (normalizedKeyID, txID, error)
func (v *VerusClient) CommitLMSIndexWithPubkeyHash(identityName, pubkeyHashHex, lmsIndex string) (string, string, error) {
	// Pre-compute the normalized VDXF ID for the pubkey_hash
	normalizedKeyID, err := v.GetVDXFID(pubkeyHashHex)
	if err != nil {
		return "", "", fmt.Errorf("failed to compute normalized key ID: %v", err)
	}

	// Commit using the original pubkey_hash (Verus will normalize it internally)
	txID, err := v.UpdateIdentity(identityName, pubkeyHashHex, lmsIndex)
	if err != nil {
		return "", "", err
	}

	return normalizedKeyID, txID, nil
}

// GetLatestLMSIndexByPubkeyHash gets the latest committed LMS index for a pubkey_hash
func (v *VerusClient) GetLatestLMSIndexByPubkeyHash(identityName, pubkeyHashHex string) (string, error) {
	// Compute normalized key ID
	normalizedKeyID, err := v.GetVDXFID(pubkeyHashHex)
	if err != nil {
		return "", fmt.Errorf("failed to compute normalized key ID: %v", err)
	}

	// Query using normalized key ID
	return v.GetLatestIndexForKey(identityName, normalizedKeyID)
}
