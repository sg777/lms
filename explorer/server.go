package explorer

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ExplorerServer provides web interface for exploring hash chains
type ExplorerServer struct {
	raftEndpoints   []string
	hsmEndpoint     string // HSM server endpoint
	port            int
	client          *http.Client
	authServer      *AuthServer
	walletDB        *WalletDB        // Wallet database
	keyBlockchainDB *KeyBlockchainDB // Key blockchain settings database

	// Cache for recent commits
	cacheMu          sync.RWMutex
	recentCommits    []CommitInfo
	cacheLastUpdated time.Time
	cacheTTL         time.Duration
}

// CommitInfo represents a single commit entry for display
type CommitInfo struct {
	KeyID        string    `json:"key_id"`
	PubkeyHash   string    `json:"pubkey_hash"`
	Index        uint64    `json:"index"`
	Hash         string    `json:"hash"`
	PreviousHash string    `json:"previous_hash"`
	RaftIndex    uint64    `json:"raft_index,omitempty"`
	Timestamp    time.Time `json:"timestamp,omitempty"`
}

// ChainEntry represents a chain entry with verification status
type ChainEntry struct {
	KeyID        string `json:"key_id"`
	PubkeyHash   string `json:"pubkey_hash"`
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
	KeyID      string       `json:"key_id"`
	Count      int          `json:"count"`
	Entries    []ChainEntry `json:"entries"`
	Valid      bool         `json:"valid"`
	Error      string       `json:"error,omitempty"`
	BreakIndex int          `json:"break_index,omitempty"`
}

// SearchResponse represents search results
type SearchResponse struct {
	Type    string         `json:"type"` // "key_id", "hash", "not_found"
	KeyID   string         `json:"key_id,omitempty"`
	Entry   *ChainEntry    `json:"entry,omitempty"`
	Chain   *ChainResponse `json:"chain,omitempty"`
	Message string         `json:"message,omitempty"`
}

// StatsResponse represents overall statistics
type StatsResponse struct {
	TotalKeys    int       `json:"total_keys"`
	TotalCommits int       `json:"total_commits"`
	ValidChains  int       `json:"valid_chains"`
	BrokenChains int       `json:"broken_chains"`
	LastCommit   time.Time `json:"last_commit,omitempty"`
}

// NewExplorerServer creates a new explorer server
func NewExplorerServer(port int, raftEndpoints []string, hsmEndpoint string) (*ExplorerServer, error) {
	authServer, err := NewAuthServer()
	if err != nil {
		return nil, fmt.Errorf("failed to create auth server: %v", err)
	}

	walletDB, err := NewWalletDB("explorer/wallets.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet database: %v", err)
	}

	keyBlockchainDB, err := NewKeyBlockchainDB("explorer/key_blockchain.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create key blockchain database: %v", err)
	}

	return &ExplorerServer{
		raftEndpoints: raftEndpoints,
		hsmEndpoint:   hsmEndpoint,
		port:          port,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		authServer:      authServer,
		walletDB:        walletDB,
		keyBlockchainDB: keyBlockchainDB,
		cacheTTL:        5 * time.Second, // Cache for 5 seconds
	}, nil
}

// loggingMiddleware logs all incoming requests and recovers from panics
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[HTTP_REQUEST] PANIC in handler for %s %s: %v", r.Method, r.URL.String(), rec)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("Internal server error: %v", rec),
				})
			}
		}()
		log.Printf("[HTTP_REQUEST] %s %s (Path: %s, RawPath: %s) from %s", r.Method, r.URL.String(), r.URL.Path, r.URL.RawPath, r.RemoteAddr)
		next.ServeHTTP(w, r)
	})
}

// Start starts the explorer server
func (s *ExplorerServer) Start() error {
	// Note: HSM server validation is done per-request, not at startup
	// This allows explorer to start for browsing/login even if HSM server is down
	// HSM operations (generate key, sign, etc.) will fail gracefully with error messages

	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/recent", s.handleRecent)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/chain/", s.handleChain)          // /api/chain/<key_id>
	mux.HandleFunc("/api/blockchain", s.handleBlockchain) // /api/blockchain - all blockchain commits

	// Authentication endpoints
	mux.HandleFunc("/api/auth/register", s.authServer.Register)
	mux.HandleFunc("/api/auth/login", s.authServer.Login)
	mux.HandleFunc("/api/auth/me", s.authServer.GetMe)

	// Authenticated HSM endpoints (proxy to HSM server with user context)
	mux.HandleFunc("/api/my/keys", s.handleMyKeys)
	mux.HandleFunc("/api/my/generate", s.handleGenerateKey)
	mux.HandleFunc("/api/my/sign", s.handleSign)
	mux.HandleFunc("/api/my/verify", s.handleVerify)
	mux.HandleFunc("/api/my/export", s.handleExportKey)
	mux.HandleFunc("/api/my/import", s.handleImportKey)
	mux.HandleFunc("/api/my/delete", s.handleDeleteKey)

	// Wallet endpoints
	log.Printf("[SERVER_SETUP] Registering wallet endpoints...")

	// List endpoint (with trailing slash support)
	listHandler := http.HandlerFunc(s.handleWalletList)
	mux.Handle("/api/my/wallet/list", listHandler)
	mux.Handle("/api/my/wallet/list/", listHandler)

	// Create endpoint (with trailing slash support)
	createHandler := http.HandlerFunc(s.handleWalletCreate)
	mux.Handle("/api/my/wallet/create", createHandler)
	mux.Handle("/api/my/wallet/create/", createHandler)

	// CRITICAL: Register balance endpoint BEFORE catch-all
	log.Printf("[SERVER_SETUP] Registering /api/my/wallet/balance -> handleWalletBalance")
	balanceHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[WALLET_BALANCE_ROUTE] ✅ Route handler called! Path: %s, RawPath: %s, Query: %s", r.URL.Path, r.URL.RawPath, r.URL.RawQuery)
		s.handleWalletBalance(w, r)
	})
	mux.Handle("/api/my/wallet/balance", balanceHandler)
	mux.Handle("/api/my/wallet/balance/", balanceHandler) // Also handle trailing slash

	// Total balance endpoint (with trailing slash support)
	log.Printf("[SERVER_SETUP] Registering /api/my/wallet/total-balance -> handleWalletTotalBalance")
	totalBalanceHandler := http.HandlerFunc(s.handleWalletTotalBalance)
	mux.Handle("/api/my/wallet/total-balance", totalBalanceHandler)
	mux.Handle("/api/my/wallet/total-balance/", totalBalanceHandler) // FIX: Handle trailing slash

	log.Printf("[SERVER_SETUP] ✅ Wallet endpoints registered: /api/my/wallet/list, /api/my/wallet/create, /api/my/wallet/balance, /api/my/wallet/total-balance (all with trailing slash support)")

	// Key blockchain endpoints
	mux.HandleFunc("/api/my/key/blockchain/toggle", s.handleKeyBlockchainToggle)
	mux.HandleFunc("/api/my/key/blockchain/status", s.handleKeyBlockchainStatus)

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./explorer/static"))))

	// CRITICAL: Catch-all MUST be last - serves index.html for SPA routing
	// But returns JSON error for any unmatched API routes
	log.Printf("[SERVER_SETUP] Registering catch-all route / -> handleIndex (LAST)")
	mux.HandleFunc("/", s.handleIndex)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("========================================")
	log.Printf("Explorer server starting on http://localhost%s", addr)
	log.Printf("HSM server endpoint: %s", s.hsmEndpoint)
	log.Printf("Connecting to Raft endpoints: %v", s.raftEndpoints)
	log.Printf("Note: Explorer can start without HSM server. HSM operations will fail if server is unavailable.")
	log.Printf("[VERSION] Server built with enhanced logging - all requests will be logged")
	log.Printf("========================================")

	// Wrap mux with logging middleware
	handler := loggingMiddleware(mux)
	return http.ListenAndServe(addr, handler)
}

// handleIndex serves the main HTML page
// WARNING: This is the catch-all handler - it should be registered LAST
func (s *ExplorerServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// CRITICAL: NEVER serve HTML for API routes - return JSON error instead
	if strings.HasPrefix(r.URL.Path, "/api/") {
		log.Printf("[HANDLE_INDEX] ❌ ERROR: API route caught by catch-all handler! This means the route wasn't registered or didn't match.")
		log.Printf("[HANDLE_INDEX] Request details - Path: %s, RawPath: %s, Method: %s", r.URL.Path, r.URL.RawPath, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("API endpoint not found: %s. This route was not registered or the request didn't match any registered route.", r.URL.Path),
		})
		return
	}

	// Only serve index.html for non-API routes (SPA routing)
	if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/static/") {
		log.Printf("[HANDLE_INDEX] Serving index.html for SPA route: %s", r.URL.Path)
	}
	http.ServeFile(w, r, "./explorer/templates/index.html")
}
