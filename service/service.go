package service

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// Service wraps the Raft cluster and provides the API layer
type Service struct {
	raft    *raft.Raft
	api     *APIServer
	config  *Config
	fsm     FSMInterface
}

// NewService creates and initializes a new service
func NewService(cfg *Config, fsm FSMInterface) (*Service, error) {
	// Create Raft data directory
	raftDir := filepath.Join(cfg.RaftDir, cfg.NodeID)
	if err := os.MkdirAll(raftDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create raft directory: %v", err)
	}

	// Define BoltDB path
	dbPath := filepath.Join(raftDir, "raft.db")

	// Create BoltDB store
	boltStore, err := raftboltdb.New(raftboltdb.Options{
		Path: dbPath,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create BoltDB store: %v", err)
	}

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Set up Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(cfg.NodeID)
	
	// Reduce timeouts for faster failover
	config.HeartbeatTimeout = 500 * time.Millisecond
	config.ElectionTimeout = 500 * time.Millisecond
	config.LeaderLeaseTimeout = 500 * time.Millisecond

	// Create transport for Raft communication
	addr, err := net.ResolveTCPAddr("tcp", cfg.NodeAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %v", err)
	}
	
	transport, err := raft.NewTCPTransport(
		"0.0.0.0:7000",
		addr,
		3,
		10*time.Second,
		os.Stderr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %v", err)
	}

	// Create Raft node
	r, err := raft.NewRaft(config, fsm, boltStore, boltStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create Raft node: %v", err)
	}

	// Bootstrap cluster if needed
	if cfg.Bootstrap {
		log.Println("Bootstrapping cluster...")
		servers := make([]raft.Server, 0, len(cfg.ClusterNodes))
		for _, node := range cfg.ClusterNodes {
			servers = append(servers, raft.Server{
				ID:      raft.ServerID(node.ID),
				Address: raft.ServerAddress(node.Address),
			})
		}
		configuration := raft.Configuration{Servers: servers}
		bootstrapFuture := r.BootstrapCluster(configuration)
		if err := bootstrapFuture.Error(); err != nil {
			return nil, fmt.Errorf("failed to bootstrap cluster: %v", err)
		}
		log.Println("Cluster bootstrapped successfully")
	}

	// Create API server
	api := NewAPIServer(r, fsm, cfg)

	return &Service{
		raft:   r,
		api:    api,
		config: cfg,
		fsm:    fsm,
	}, nil
}

// Start starts the service (API server)
func (s *Service) Start() error {
	// Log leadership changes
	go func() {
		for {
			select {
			case leader := <-s.raft.LeaderCh():
				if leader {
					log.Printf("Node %s is now the leader", s.config.NodeID)
				} else {
					log.Printf("Node %s lost leadership", s.config.NodeID)
				}
			}
		}
	}()

	// Start API server
	return s.api.Start()
}

// Shutdown gracefully shuts down the service
func (s *Service) Shutdown() error {
	log.Println("Shutting down service...")
	
	// Shutdown Raft
	future := s.raft.Shutdown()
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to shutdown Raft: %v", err)
	}
	
	log.Println("Service shut down successfully")
	return nil
}

// GetRaft returns the Raft instance (for testing/debugging)
func (s *Service) GetRaft() *raft.Raft {
	return s.raft
}

// RunServiceFromFlags creates and runs a service from command-line flags
func RunServiceFromFlags(fsm FSMInterface) error {
	// Parse flags
	nodeID := flag.String("id", "node1", "Node ID (e.g., node1, node2, node3)")
	nodeAddr := flag.String("addr", "159.69.23.29:7000", "Node address (IP:port)")
	apiPort := flag.Int("api-port", 8080, "API server port")
	raftPort := flag.Int("raft-port", 7000, "Raft transport port")
	raftDir := flag.String("raft-dir", "./raft-data", "Raft data directory")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster")
	flag.Parse()

	// Create config
	cfg := DefaultConfig()
	cfg.NodeID = *nodeID
	cfg.NodeAddr = *nodeAddr
	cfg.APIPort = *apiPort
	cfg.RaftPort = *raftPort
	cfg.RaftDir = *raftDir
	cfg.Bootstrap = *bootstrap

	// Create and start service
	service, err := NewService(cfg, fsm)
	if err != nil {
		return fmt.Errorf("failed to create service: %v", err)
	}

	log.Printf("Starting service: node=%s, raft=%s, api=:%d", *nodeID, *nodeAddr, *apiPort)
	
	// Start service (blocks)
	if err := service.Start(); err != nil {
		return fmt.Errorf("service error: %v", err)
	}

	return nil
}

