package explorer

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ExplorerServer provides web interface for exploring hash chains
type ExplorerServer struct {
	raftEndpoints []string
	hsmEndpoint   string // HSM server endpoint
	port          int
	client        *http.Client
	authServer    *AuthServer
	
	// Cache for recent commits
	cacheMu          sync.RWMutex
	recentCommits    []CommitInfo
	cacheLastUpdated time.Time
	cacheTTL         time.Duration
}

// CommitInfo represents a single commit entry for display
type CommitInfo struct {
	KeyID        string    `json:"key_id"`
	Index        uint64    `json:"index"`
	Hash         string    `json:"hash"`
	PreviousHash string    `json:"previous_hash"`
	RaftIndex    uint64    `json:"raft_index,omitempty"`
	Timestamp    time.Time `json:"timestamp,omitempty"`
}

// ChainEntry represents a chain entry with verification status
type ChainEntry struct {
	KeyID        string `json:"key_id"`
	Index        uint64 `json:"index"`
	PreviousHash string `json:"previous_hash"`
	Hash         string `json:"hash"`
	Signature    string `json:"signature"`
	PublicKey    string `json:"public_key"`
	IsGenesis    bool   `json:"is_genesis"`
	ChainValid   bool   `json:"chain_valid"`
	ChainError   string `json:"chain_error,omitempty"`
}

// ChainResponse represents a full chain response
type ChainResponse struct {
	KeyID        string       `json:"key_id"`
	Count        int          `json:"count"`
	Entries      []ChainEntry `json:"entries"`
	Valid        bool         `json:"valid"`
	Error        string       `json:"error,omitempty"`
	BreakIndex   int          `json:"break_index,omitempty"`
}

// SearchResponse represents search results
type SearchResponse struct {
	Type      string      `json:"type"` // "key_id", "hash", "not_found"
	KeyID     string      `json:"key_id,omitempty"`
	Entry     *ChainEntry `json:"entry,omitempty"`
	Chain     *ChainResponse `json:"chain,omitempty"`
	Message   string      `json:"message,omitempty"`
}

// StatsResponse represents overall statistics
type StatsResponse struct {
	TotalKeys     int `json:"total_keys"`
	TotalCommits  int `json:"total_commits"`
	ValidChains   int `json:"valid_chains"`
	BrokenChains  int `json:"broken_chains"`
	LastCommit    time.Time `json:"last_commit,omitempty"`
}

// NewExplorerServer creates a new explorer server
func NewExplorerServer(port int, raftEndpoints []string, hsmEndpoint string) (*ExplorerServer, error) {
	authServer, err := NewAuthServer()
	if err != nil {
		return nil, fmt.Errorf("failed to create auth server: %v", err)
	}

	return &ExplorerServer{
		raftEndpoints: raftEndpoints,
		hsmEndpoint:   hsmEndpoint,
		port:          port,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		authServer: authServer,
		cacheTTL:   5 * time.Second, // Cache for 5 seconds
	}, nil
}

// Start starts the explorer server
func (s *ExplorerServer) Start() error {
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/api/recent", s.handleRecent)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/chain/", s.handleChain) // /api/chain/<key_id>
	
	// Authentication endpoints
	mux.HandleFunc("/api/auth/register", s.authServer.Register)
	mux.HandleFunc("/api/auth/login", s.authServer.Login)
	mux.HandleFunc("/api/auth/me", s.authServer.GetMe)
	
	// Authenticated HSM endpoints (proxy to HSM server with user context)
	mux.HandleFunc("/api/my/keys", s.handleMyKeys)
	mux.HandleFunc("/api/my/generate", s.handleGenerateKey)
	mux.HandleFunc("/api/my/sign", s.handleSign)
	mux.HandleFunc("/api/my/export", s.handleExportKey)
	mux.HandleFunc("/api/my/import", s.handleImportKey)
	mux.HandleFunc("/api/my/delete", s.handleDeleteKey)
	
	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./explorer/static"))))
	
	// Serve index.html for root and other routes (SPA routing)
	mux.HandleFunc("/", s.handleIndex)
	
	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Explorer server starting on http://localhost%s", addr)
	log.Printf("Connecting to Raft endpoints: %v", s.raftEndpoints)
	
	return http.ListenAndServe(addr, mux)
}

// handleIndex serves the main HTML page
func (s *ExplorerServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./explorer/templates/index.html")
}

