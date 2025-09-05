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
	networkMgr    *NetworkManager
	eventBus      EventBus
	ctx           context.Context
	cancel        context.CancelFunc
	network       string
	liveMode      bool
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
	networkMgr := NewNetworkManager(network, eventBus)
	
	// Enable live mode for real network connectivity
	liveMode := os.Getenv("BITCOIN_LIVE_MODE") == "true"
	
	return &SentinelServer{
		bitcoinProto: bitcoinProto,
		networkMgr:   networkMgr,
		eventBus:     eventBus,
		ctx:          ctx,
		cancel:       cancel,
		network:      network,
		liveMode:     liveMode,
	}
}

// Start begins the real Bitcoin SV P2P networking service
func (s *SentinelServer) Start() error {
	if s.liveMode {
		log.Printf("🌐 Sentinel P2P Service: Starting LIVE Bitcoin SV %s networking...", s.network)
		
		// Connect to live Bitcoin SV network
		if err := s.networkMgr.ConnectToNetwork(); err != nil {
			log.Printf("⚠️  Warning: Live network connection failed: %v", err)
			// Fall back to protocol-only mode
			s.liveMode = false
		}
	}
	
	if !s.liveMode {
		log.Printf("🌐 Sentinel P2P Service: Starting Bitcoin SV %s protocol (demo mode)...", s.network)
		
		// Use protocol handler for demo/testing
		if err := s.bitcoinProto.ConnectToPeers(); err != nil {
			log.Printf("⚠️  Warning: Protocol peer connection failed: %v", err)
		}
	}
	
	// Start peer management and monitoring
	go s.managePeers()
	go s.monitorHealth()
	
	mode := "demo"
	if s.liveMode {
		mode = "LIVE"
	}
	log.Printf("🌐 Sentinel: Bitcoin SV P2P service started on %s (%s mode)", s.network, mode)
	return nil
}

// managePeers handles Bitcoin SV peer connection lifecycle for both live and demo modes
func (s *SentinelServer) managePeers() {
	ticker := time.NewTicker(60 * time.Second) // Check peer health every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			var peerCount int
			var status map[string]interface{}
			
			if s.liveMode {
				// Get live network status
				status = s.networkMgr.GetNetworkStatus()
				peerCount = status["peer_count"].(int)
				log.Printf("🌐 Sentinel: Managing %d LIVE Bitcoin SV peers", peerCount)
				
				// Check if we need more live peers
				minPeers := status["min_peers"].(int)
				if peerCount < minPeers {
					log.Printf("🔄 Sentinel: Below minimum live peers (%d), network manager will discover more...", minPeers)
				}
			} else {
				// Demo mode - use protocol handler
				peerCount = len(s.bitcoinProto.peers)
				log.Printf("🌐 Sentinel: Managing %d demo Bitcoin SV peers", peerCount)
				
				// If we have too few peers, try to connect to more
				if peerCount < 3 {
					log.Println("🔄 Sentinel: Low peer count, attempting to connect to more peers...")
					go s.bitcoinProto.ConnectToPeers()
				}
				
				status = map[string]interface{}{
					"peer_count": peerCount,
					"network":    s.network,
					"mode":       "demo",
				}
			}
			
			// Publish peer status event
			status["timestamp"] = time.Now().Unix()
			status["live_mode"] = s.liveMode
			s.eventBus.Publish("p2p.peer_status.v1", status)
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
	mode := "demo"
	if s.liveMode {
		mode = "LIVE"
	}
	log.Printf("🛑 Sentinel: Shutting down Bitcoin SV P2P service (%s mode)...", mode)
	
	s.cancel()
	
	// Stop live network manager if running
	if s.liveMode && s.networkMgr != nil {
		if err := s.networkMgr.Stop(); err != nil {
			log.Printf("⚠️  Error stopping network manager: %v", err)
		}
	}
	
	log.Printf("🛑 Sentinel: Bitcoin SV P2P service stopped (%s mode)", mode)
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
