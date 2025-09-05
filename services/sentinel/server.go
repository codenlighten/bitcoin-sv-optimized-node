package sentinel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"sync"
	"time"

)



// SentinelServer implements the Sentinel P2P networking service
// Handles real Bitcoin SV P2P connections, message routing, and network health
type SentinelServer struct {
	mu            sync.RWMutex
	bitcoinProto  *BitcoinProtocol
	eventBus      EventBus
	ctx           context.Context
	cancel        context.CancelFunc
	network       string
}

// NewSentinelServer creates a new Sentinel P2P service with real Bitcoin protocol
func NewSentinelServer(eventBus EventBus) *SentinelServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Determine network from environment (default to testnet for safety)
	network := os.Getenv("BITCOIN_NETWORK")
	if network == "" {
		network = "testnet" // Default to testnet for development
	}
	
	bitcoinProto := NewBitcoinProtocol(network, eventBus)
	
	return &SentinelServer{
		bitcoinProto: bitcoinProto,
		eventBus:     eventBus,
		ctx:          ctx,
		cancel:       cancel,
		network:      network,
	}
}

// Start begins the real Bitcoin SV P2P networking service
func (s *SentinelServer) Start() error {
	log.Printf("🌐 Sentinel P2P Service: Starting Bitcoin SV %s networking...", s.network)
	
	// Connect to Bitcoin SV network peers
	if err := s.bitcoinProto.ConnectToPeers(); err != nil {
		log.Printf("⚠️  Warning: Initial peer connection failed: %v", err)
		// Continue anyway - peers may connect later
	}
	
	// Start peer management and monitoring
	go s.managePeers()
	go s.monitorHealth()
	
	log.Printf("🌐 Sentinel: Real Bitcoin SV P2P service started on %s", s.network)
	return nil
}

// managePeers handles real Bitcoin SV peer connection lifecycle
func (s *SentinelServer) managePeers() {
	ticker := time.NewTicker(60 * time.Second) // Check peer health every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Monitor peer connections and reconnect if needed
			peerCount := len(s.bitcoinProto.peers)
			log.Printf("🌐 Sentinel: Managing %d Bitcoin SV peers", peerCount)
			
			// If we have too few peers, try to connect to more
			if peerCount < 3 {
				log.Println("🔄 Sentinel: Low peer count, attempting to connect to more peers...")
				go s.bitcoinProto.ConnectToPeers()
			}
			
			// Publish peer status event
			s.eventBus.Publish("p2p.peer_status.v1", map[string]interface{}{
				"peer_count": peerCount,
				"network":    s.network,
				"timestamp":  time.Now().Unix(),
			})
		}
	}
}

// monitorHealth monitors the health of the Bitcoin SV P2P service
func (s *SentinelServer) monitorHealth() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Monitor service health
			peerCount := len(s.bitcoinProto.peers)
			status := "healthy"
			if peerCount == 0 {
				status = "no_peers"
			} else if peerCount < 3 {
				status = "low_peers"
			}
			
			// Publish health status
			s.eventBus.Publish("p2p.health.v1", map[string]interface{}{
				"status":     status,
				"peer_count": peerCount,
				"network":    s.network,
				"timestamp":  time.Now().Unix(),
			})
		}
	}
}

// Stop gracefully shuts down the Sentinel service
func (s *SentinelServer) Stop() error {
	log.Println("🛑 Sentinel: Shutting down Bitcoin SV P2P service...")
	s.cancel()
	return nil
}

func generatePeerID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateTraceID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
