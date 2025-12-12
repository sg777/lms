package simulator

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/verifiable-state-chains/lms/client"
	"github.com/verifiable-state-chains/lms/models"
)

// HSMSimulator simulates an HSM partition
type HSMSimulator struct {
	id            string
	client        *client.HSMClient
	protocol      *client.HSMProtocol
	genesisHash   string
	mu            sync.Mutex
	attestations  []*models.AttestationResponse
	errors        []error
	stats         *HSMStats
}

// HSMStats tracks statistics for an HSM simulator
type HSMStats struct {
	TotalAttestations   int
	SuccessfulCommits   int
	FailedCommits       int
	UnusableIndices     int
	LastLMSIndex        uint64
	LastSequenceNumber  uint64
}

// NewHSMSimulator creates a new HSM simulator
func NewHSMSimulator(
	id string,
	serviceEndpoints []string,
	genesisHash string,
) *HSMSimulator {
	hsmClient := client.NewHSMClient(serviceEndpoints, id)
	protocol := client.NewHSMProtocol(hsmClient, genesisHash)

	return &HSMSimulator{
		id:           id,
		client:       hsmClient,
		protocol:     protocol,
		genesisHash:  genesisHash,
		attestations: make([]*models.AttestationResponse, 0),
		errors:       make([]error, 0),
		stats:        &HSMStats{},
	}
}

// GetID returns the HSM identifier
func (h *HSMSimulator) GetID() string {
	return h.id
}

// GetStats returns the current statistics
func (h *HSMSimulator) GetStats() *HSMStats {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Create a copy
	stats := *h.stats
	return &stats
}

// GetAttestations returns all committed attestations
func (h *HSMSimulator) GetAttestations() []*models.AttestationResponse {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	// Return copies
	attestations := make([]*models.AttestationResponse, len(h.attestations))
	for i, att := range h.attestations {
		data, _ := att.ToJSON()
		var copy models.AttestationResponse
		copy.FromJSON(data)
		attestations[i] = &copy
	}
	return attestations
}

// GetErrors returns all errors encountered
func (h *HSMSimulator) GetErrors() []error {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	errors := make([]error, len(h.errors))
	copy(errors, h.errors)
	return errors
}

// SyncState synchronizes the HSM's state with the service
func (h *HSMSimulator) SyncState() error {
	return h.protocol.SyncState()
}

// GenerateAttestation generates and commits a new attestation
// Returns true if successful, false if failed
func (h *HSMSimulator) GenerateAttestation(messageHash string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Sync state first
	if err := h.protocol.SyncState(); err != nil {
		h.errors = append(h.errors, fmt.Errorf("sync state failed: %v", err))
		h.stats.FailedCommits++
		return false, err
	}

	// Get next usable index
	nextIndex := h.protocol.GetNextUsableIndex()
	if !h.protocol.IsIndexUsable(nextIndex) {
		err := fmt.Errorf("index %d is unusable", nextIndex)
		h.errors = append(h.errors, err)
		h.stats.FailedCommits++
		h.stats.UnusableIndices++
		return false, err
	}

	// Create attestation payload
	timestamp := time.Now().UTC().Format(time.RFC3339)
	payload, err := h.protocol.CreateAttestationPayload(
		nextIndex,
		messageHash,
		timestamp,
		fmt.Sprintf("hsm=%s", h.id),
	)
	if err != nil {
		h.errors = append(h.errors, err)
		h.stats.FailedCommits++
		return false, err
	}

	// Generate mock signature and certificate
	signature := h.generateMockSignature(payload)
	certificate := h.generateMockCertificate(h.id, nextIndex)

	// Create attestation response
	attestation, err := h.protocol.CreateAttestationResponse(
		payload,
		"LMS_ATTEST_POLICY",
		"PS256",
		signature,
		certificate,
	)
	if err != nil {
		h.errors = append(h.errors, err)
		h.stats.FailedCommits++
		return false, err
	}

	// Commit attestation
	committed, raftIndex, raftTerm, err := h.protocol.CommitAttestation(
		attestation,
		5*time.Second,
	)

	if err != nil || !committed {
		h.errors = append(h.errors, err)
		h.stats.FailedCommits++
		if err == nil {
			err = fmt.Errorf("attestation not committed")
		}
		return false, err
	}

	// Success!
	h.attestations = append(h.attestations, attestation)
	h.stats.TotalAttestations++
	h.stats.SuccessfulCommits++
	h.stats.LastLMSIndex = payload.LMSIndex
	h.stats.LastSequenceNumber = payload.SequenceNumber

	log.Printf("[HSM %s] Successfully committed attestation: LMS Index=%d, Sequence=%d, Raft Index=%d, Term=%d",
		h.id, payload.LMSIndex, payload.SequenceNumber, raftIndex, raftTerm)

	return true, nil
}

// GenerateAttestations generates multiple attestations sequentially
func (h *HSMSimulator) GenerateAttestations(count int, messageHashPrefix string) error {
	for i := 0; i < count; i++ {
		messageHash := fmt.Sprintf("%s-%d", messageHashPrefix, i)
		success, err := h.GenerateAttestation(messageHash)
		if !success {
			return fmt.Errorf("failed to generate attestation %d: %v", i, err)
		}
		// Small delay between attestations
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// generateMockSignature generates a mock signature for testing
func (h *HSMSimulator) generateMockSignature(payload *models.ChainedPayload) string {
	// In real HSM, this would be a cryptographic signature
	// For simulation, we generate random bytes
	sig := make([]byte, 64)
	rand.Read(sig)
	return base64.StdEncoding.EncodeToString(sig)
}

// generateMockCertificate generates a mock certificate for testing
func (h *HSMSimulator) generateMockCertificate(hsmID string, lmsIndex uint64) string {
	// In real HSM, this would be a proper X.509 certificate
	// For simulation, we generate a mock PEM
	certPEM := fmt.Sprintf(`-----BEGIN CERTIFICATE-----
MOCK CERTIFICATE
HSM ID: %s
LMS Index: %d
Timestamp: %s
-----END CERTIFICATE-----`, hsmID, lmsIndex, time.Now().UTC().Format(time.RFC3339))
	return base64.StdEncoding.EncodeToString([]byte(certPEM))
}

// HSMSimulatorPool manages multiple HSM simulators
type HSMSimulatorPool struct {
	simulators []*HSMSimulator
	mu         sync.RWMutex
}

// NewHSMSimulatorPool creates a new pool of HSM simulators
func NewHSMSimulatorPool(
	serviceEndpoints []string,
	genesisHash string,
	count int,
) *HSMSimulatorPool {
	pool := &HSMSimulatorPool{
		simulators: make([]*HSMSimulator, 0, count),
	}

	for i := 0; i < count; i++ {
		hsmID := fmt.Sprintf("hsm-sim-%d", i+1)
		sim := NewHSMSimulator(hsmID, serviceEndpoints, genesisHash)
		pool.simulators = append(pool.simulators, sim)
	}

	return pool
}

// GetSimulator returns a simulator by index
func (p *HSMSimulatorPool) GetSimulator(index int) *HSMSimulator {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	if index < 0 || index >= len(p.simulators) {
		return nil
	}
	return p.simulators[index]
}

// GetAllSimulators returns all simulators
func (p *HSMSimulatorPool) GetAllSimulators() []*HSMSimulator {
	p.mu.RLock()
	defer p.mu.RUnlock()
	
	simulators := make([]*HSMSimulator, len(p.simulators))
	copy(simulators, p.simulators)
	return simulators
}

// GetCount returns the number of simulators
func (p *HSMSimulatorPool) GetCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.simulators)
}

// RunConcurrentAttestations runs attestations concurrently across all HSMs
func (p *HSMSimulatorPool) RunConcurrentAttestations(
	attestationsPerHSM int,
	messageHashPrefix string,
) error {
	var wg sync.WaitGroup
	errors := make(chan error, len(p.simulators)*attestationsPerHSM)

	simulators := p.GetAllSimulators()

	for _, sim := range simulators {
		wg.Add(1)
		go func(s *HSMSimulator) {
			defer wg.Done()
			for i := 0; i < attestationsPerHSM; i++ {
				messageHash := fmt.Sprintf("%s-%s-%d", messageHashPrefix, s.GetID(), i)
				success, err := s.GenerateAttestation(messageHash)
				if !success {
					errors <- fmt.Errorf("HSM %s failed attestation %d: %v", s.GetID(), i, err)
				} else {
					// Small delay
					time.Sleep(50 * time.Millisecond)
				}
			}
		}(sim)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var errList []error
	for err := range errors {
		errList = append(errList, err)
	}

	if len(errList) > 0 {
		return fmt.Errorf("encountered %d errors: %v", len(errList), errList[0])
	}

	return nil
}

// GetTotalStats returns aggregated statistics from all HSMs
func (p *HSMSimulatorPool) GetTotalStats() map[string]*HSMStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make(map[string]*HSMStats)
	for _, sim := range p.simulators {
		stats[sim.GetID()] = sim.GetStats()
	}
	return stats
}

