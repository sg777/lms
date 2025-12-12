package models

// API Request/Response types for the HSM client protocol

// GetLatestHeadRequest requests the latest attestation head from the service
type GetLatestHeadRequest struct {
	HSMIdentifier string `json:"hsm_identifier"`
}

// GetLatestHeadResponse returns the latest committed attestation
type GetLatestHeadResponse struct {
	Success      bool                `json:"success"`
	Attestation  *AttestationResponse `json:"attestation,omitempty"`
	Error        string              `json:"error,omitempty"`
	RaftIndex    uint64              `json:"raft_index,omitempty"`    // Raft log index
	RaftTerm     uint64              `json:"raft_term,omitempty"`      // Raft term
}

// ProposeAttestationRequest proposes a new attestation to be committed
type ProposeAttestationRequest struct {
	Attestation *AttestationResponse `json:"attestation"`
	HSMIdentifier string             `json:"hsm_identifier"`
}

// ProposeAttestationResponse confirms whether the attestation was committed
type ProposeAttestationResponse struct {
	Success     bool   `json:"success"`
	Committed   bool   `json:"committed"`   // Whether it was actually committed
	RaftIndex   uint64 `json:"raft_index,omitempty"`   // Raft log index if committed
	RaftTerm    uint64 `json:"raft_term,omitempty"`    // Raft term if committed
	Error       string `json:"error,omitempty"`
	Message     string `json:"message,omitempty"`      // Additional info
}

// HealthCheckRequest checks if the service is healthy
type HealthCheckRequest struct{}

// HealthCheckResponse returns service health status
type HealthCheckResponse struct {
	Healthy   bool   `json:"healthy"`
	Leader    string `json:"leader,omitempty"`    // Current leader ID
	IsLeader  bool   `json:"is_leader"`          // Whether this node is the leader
	Term      uint64 `json:"term,omitempty"`      // Current Raft term
	Error     string `json:"error,omitempty"`
}

// LeaderInfoRequest requests information about the current leader
type LeaderInfoRequest struct{}

// LeaderInfoResponse returns leader information
type LeaderInfoResponse struct {
	LeaderID   string `json:"leader_id"`
	LeaderAddr string `json:"leader_addr,omitempty"`
	IsLeader   bool   `json:"is_leader"`
	Error      string `json:"error,omitempty"`
}

