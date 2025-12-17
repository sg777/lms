package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	
	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/models"
	"github.com/verifiable-state-chains/lms/validation"
)

// APIServer provides HTTP API for HSM clients
type APIServer struct {
	raft          *raft.Raft
	forwarder     *LeaderForwarder
	fsm           FSMInterface
	config        *Config
	validator     *validation.AttestationValidator
}

// FSMInterface defines the interface our FSM must implement
// It must implement raft.FSM plus additional methods
type FSMInterface interface {
	raft.FSM
	GetLatestAttestation() (*models.AttestationResponse, error)
	GetLogEntry(index uint64) (*models.LogEntry, error)
	GetLogCount() uint64
	GetSimpleMessages() []string
	GetAllLogEntries() []*models.LogEntry
	GetGenesisHash() string
	// Optional: KeyIndexFSMInterface methods
	GetKeyIndex(keyID string) (uint64, bool)
	GetAllKeyIndices() map[string]uint64
}

// NewAPIServer creates a new API server
func NewAPIServer(r *raft.Raft, fsm FSMInterface, cfg *Config, genesisHash string) *APIServer {
	forwarder := NewLeaderForwarder(r, cfg)
	validator := validation.NewAttestationValidator(genesisHash)
	// Use mock signature verifier for now (can be replaced with real crypto)
	validator.SetSignatureVerifier(validation.MockSignatureVerifier())
	return &APIServer{
		raft:      r,
		forwarder: forwarder,
		fsm:       fsm,
		config:    cfg,
		validator: validator,
	}
}

// Start starts the HTTP API server
func (s *APIServer) Start() error {
	mux := http.NewServeMux()
	
	// Register handlers
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/leader", s.handleLeader)
	mux.HandleFunc("/latest-head", s.handleLatestHead)
	mux.HandleFunc("/propose", s.handlePropose)
	mux.HandleFunc("/list", s.handleList)
	mux.HandleFunc("/send", s.handleSend)
	mux.HandleFunc("/keys", s.handleKeys) // Get all key IDs
	mux.HandleFunc("/key/", s.handleKeyIndex) // /key/<key_id>/index (backward compatibility)
	mux.HandleFunc("/pubkey_hash/", s.handlePubkeyHashIndex) // /pubkey_hash/<pubkey_hash>/index (Phase B)
	mux.HandleFunc("/commit_index", s.handleCommitIndex)
	mux.HandleFunc("/all_entries", s.handleAllEntries) // Get all entries ordered by Raft log index
	
	addr := fmt.Sprintf(":%d", s.config.APIPort)
	log.Printf("Starting API server on %s", addr)
	
	return http.ListenAndServe(addr, mux)
}

// handleHealth handles health check requests
func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Get term from stats
	stats := s.raft.Stats()
	term := uint64(0)
	if termStr, ok := stats["term"]; ok {
		fmt.Sscanf(termStr, "%d", &term)
	}
	
	response := models.HealthCheckResponse{
		Healthy:  s.raft.State() != raft.Shutdown,
		Leader:   s.forwarder.GetLeaderID(),
		IsLeader: s.forwarder.IsLeader(),
		Term:     term,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLeader handles leader info requests
func (s *APIServer) handleLeader(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	response := models.LeaderInfoResponse{
		LeaderID:   s.forwarder.GetLeaderID(),
		LeaderAddr: s.forwarder.GetLeaderAPIAddress(),
		IsLeader:   s.forwarder.IsLeader(),
	}
	
	if response.LeaderID == "" {
		response.Error = "No leader available"
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLatestHead handles requests for the latest attestation head
func (s *APIServer) handleLatestHead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, "/latest-head")
		return
	}
	
	// Get latest attestation from FSM
	attestation, err := s.fsm.GetLatestAttestation()
	if err != nil {
		response := models.GetLatestHeadResponse{
			Success: false,
			Error:   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Get Raft stats
	stats := s.raft.Stats()
	raftIndex := uint64(0)
	raftTerm := uint64(0)
	
	if idxStr, ok := stats["last_log_index"]; ok {
		fmt.Sscanf(idxStr, "%d", &raftIndex)
	}
	if termStr, ok := stats["term"]; ok {
		fmt.Sscanf(termStr, "%d", &raftTerm)
	}
	
	response := models.GetLatestHeadResponse{
		Success:     true,
		Attestation: attestation,
		RaftIndex:   raftIndex,
		RaftTerm:    raftTerm,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handlePropose handles attestation proposal requests (DISABLED - only LMS index commits allowed)
// This endpoint is disabled because this service only accepts LMS index-related messages
// via the /commit_index endpoint with proper EC signature authentication.
func (s *APIServer) handlePropose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// SECURITY: Reject all /propose requests - only /commit_index is allowed
	// This service only accepts LMS index commitments with proper authentication
	response := models.ProposeAttestationResponse{
		Success: false,
		Error:   "This service only accepts LMS index-related messages. Use /commit_index endpoint with proper EC signature authentication. Unauthenticated commits are not allowed.",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(response)
}

// handleList handles requests to list all log entries
func (s *APIServer) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, "/list")
		return
	}
	
	// Get all simple messages from FSM
	messages := s.fsm.GetSimpleMessages()
	logEntries := s.fsm.GetAllLogEntries()
	genesisHash := s.fsm.GetGenesisHash()
	
	// Enrich log entries with hash chain information
	enrichedEntries := make([]map[string]interface{}, 0, len(logEntries))
	for _, entry := range logEntries {
		enriched := map[string]interface{}{
			"index": entry.Index,
			"term":  entry.Term,
			"timestamp": entry.Timestamp,
		}
		
		if entry.Attestation != nil {
			// Extract hash chain information
			payload, err := entry.Attestation.GetChainedPayload()
			if err == nil {
				enriched["previous_hash"] = payload.PreviousHash
				enriched["lms_index"] = payload.LMSIndex
				enriched["sequence_number"] = payload.SequenceNumber
				enriched["message_signed"] = payload.MessageSigned
				enriched["timestamp"] = payload.Timestamp
				
				// Compute current hash
				hash, err := entry.Attestation.ComputeHash()
				if err == nil {
					enriched["hash"] = hash
				}
				
				// Mark if this links to genesis
				if payload.PreviousHash == genesisHash {
					enriched["is_genesis_link"] = true
				}
			}
			
			// Include full attestation
			enriched["attestation"] = entry.Attestation
		} else {
			enriched["type"] = "simple_message"
		}
		
		enrichedEntries = append(enrichedEntries, enriched)
	}
	
	response := map[string]interface{}{
		"success":      true,
		"total_count":  len(logEntries),
		"genesis_hash": genesisHash,
		"messages":     messages,
		"log_entries": enrichedEntries,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSend handles simple string message sending (DISABLED - only LMS index commits allowed)
// This endpoint is disabled because this service only accepts LMS index-related messages
// via the /commit_index endpoint with proper EC signature authentication.
func (s *APIServer) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// SECURITY: Reject all /send requests - only /commit_index is allowed
	// This service only accepts LMS index commitments with proper authentication
	response := map[string]interface{}{
		"success": false,
		"error":   "This service only accepts LMS index-related messages. Use /commit_index endpoint with proper EC signature authentication. Unauthenticated commits are not allowed.",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(response)
}

