package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	
	"github.com/hashicorp/raft"
	"github.com/verifiable-state-chains/lms/models"
)

// APIServer provides HTTP API for HSM clients
type APIServer struct {
	raft          *raft.Raft
	forwarder     *LeaderForwarder
	fsm           FSMInterface
	config        *Config
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
}

// NewAPIServer creates a new API server
func NewAPIServer(r *raft.Raft, fsm FSMInterface, cfg *Config) *APIServer {
	forwarder := NewLeaderForwarder(r, cfg)
	return &APIServer{
		raft:      r,
		forwarder: forwarder,
		fsm:       fsm,
		config:    cfg,
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

// handlePropose handles attestation proposal requests
func (s *APIServer) handlePropose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// If not leader, forward the request
	if !s.forwarder.IsLeader() {
		s.forwarder.ForwardRequest(w, r, "/propose")
		return
	}
	
	// Parse request
	var req models.ProposeAttestationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := models.ProposeAttestationResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	if req.Attestation == nil {
		response := models.ProposeAttestationResponse{
			Success: false,
			Error:   "Attestation is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Serialize attestation to JSON
	attestationData, err := req.Attestation.ToJSON()
	if err != nil {
		response := models.ProposeAttestationResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to serialize attestation: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Apply to Raft
	future := s.raft.Apply(attestationData, s.config.RequestTimeout)
	if err := future.Error(); err != nil {
		response := models.ProposeAttestationResponse{
			Success: false,
			Error:   fmt.Sprintf("Raft apply failed: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Get response from FSM
	responseData := future.Response()
	
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
	
	response := models.ProposeAttestationResponse{
		Success:   true,
		Committed: true,
		RaftIndex: raftIndex,
		RaftTerm:  raftTerm,
		Message:   fmt.Sprintf("Attestation committed: %v", responseData),
	}
	
	w.Header().Set("Content-Type", "application/json")
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
	
	response := map[string]interface{}{
		"success":     true,
		"total_count": len(logEntries),
		"messages":    messages,
		"log_entries": logEntries,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

