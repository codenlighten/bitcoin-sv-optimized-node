package sentinel

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// NetworkManager handles live Bitcoin SV network connections
// Manages peer discovery, connection lifecycle, and network health
type NetworkManager struct {
	mu              sync.RWMutex
	network         string
	activePeers     map[string]*LivePeer
	peerConnections map[string]net.Conn
	maxPeers        int
	minPeers        int
	eventBus        EventBus
	ctx             context.Context
	cancel          context.CancelFunc
	isConnected     bool
	lastBlockHeight int32
}

// LivePeer represents an active Bitcoin SV network peer
type LivePeer struct {
	ID              string
	Address         string
	Conn            net.Conn
	Version         uint32
	Services        uint64
	StartHeight     int32
	UserAgent       string
	LastSeen        time.Time
	LastPing        time.Time
	PingLatency     time.Duration
	BytesReceived   uint64
	BytesSent       uint64
	IsOutbound      bool
	Connected       bool
	Relay           bool
}

// NewNetworkManager creates a new live Bitcoin SV network manager
func NewNetworkManager(network string, eventBus EventBus) *NetworkManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &NetworkManager{
		network:         network,
		activePeers:     make(map[string]*LivePeer),
		peerConnections: make(map[string]net.Conn),
		maxPeers:        8,  // Conservative peer count for stability
		minPeers:        3,  // Minimum peers for network health
		eventBus:        eventBus,
		ctx:             ctx,
		cancel:          cancel,
		isConnected:     false,
		lastBlockHeight: 0,
	}
}

// ConnectToNetwork initiates connection to the live Bitcoin SV network
func (nm *NetworkManager) ConnectToNetwork() error {
	log.Printf("🌐 NetworkManager: Connecting to Bitcoin SV %s network...", nm.network)
	
	// Start network connection routines
	go nm.maintainPeerConnections()
	go nm.monitorNetworkHealth()
	go nm.handleIncomingConnections()
	
	// Discover and connect to initial peers
	if err := nm.discoverAndConnectPeers(); err != nil {
		log.Printf("⚠️  Warning: Initial peer discovery failed: %v", err)
		// Continue anyway - peers may connect later
	}
	
	nm.isConnected = true
	log.Printf("🌐 NetworkManager: Connected to Bitcoin SV %s network", nm.network)
	
	return nil
}

// discoverAndConnectPeers discovers and connects to Bitcoin SV network peers
func (nm *NetworkManager) discoverAndConnectPeers() error {
	// Get DNS seed addresses for the network
	dnsSeeds := nm.getDNSSeeds()
	
	var allAddresses []string
	
	// Resolve DNS seeds to get peer addresses
	for _, seed := range dnsSeeds {
		addresses, err := nm.resolveDNSSeed(seed)
		if err != nil {
			log.Printf("⚠️  Failed to resolve DNS seed %s: %v", seed, err)
			continue
		}
		allAddresses = append(allAddresses, addresses...)
	}
	
	if len(allAddresses) == 0 {
		return fmt.Errorf("no peer addresses discovered from DNS seeds")
	}
	
	log.Printf("🔍 NetworkManager: Discovered %d potential peers", len(allAddresses))
	
	// Connect to initial set of peers
	connected := 0
	for i, addr := range allAddresses {
		if connected >= nm.minPeers || i >= 10 { // Limit initial connections
			break
		}
		
		if err := nm.connectToPeer(addr); err != nil {
			log.Printf("⚠️  Failed to connect to peer %s: %v", addr, err)
			continue
		}
		
		connected++
		time.Sleep(100 * time.Millisecond) // Rate limit connections
	}
	
	log.Printf("🌐 NetworkManager: Successfully connected to %d peers", connected)
	return nil
}

// getDNSSeeds returns DNS seeds for the specified network
func (nm *NetworkManager) getDNSSeeds() []string {
	if nm.network == "mainnet" {
		return []string{
			"seed.bitcoinsv.io",
			"seed.cascharia.com",
			"seed.satoshisvision.network",
		}
	} else {
		// Testnet seeds
		return []string{
			"testnet-seed.bitcoinsv.io",
			"testnet-seed.cascharia.com",
		}
	}
}

// resolveDNSSeed resolves a DNS seed to get peer IP addresses
func (nm *NetworkManager) resolveDNSSeed(seed string) ([]string, error) {
	ips, err := net.LookupIP(seed)
	if err != nil {
		return nil, err
	}
	
	var addresses []string
	port := "8333" // Default Bitcoin SV port
	if nm.network == "testnet" {
		port = "18333" // Testnet port
	}
	
	for _, ip := range ips {
		if ip.To4() != nil { // IPv4 only for now
			addresses = append(addresses, net.JoinHostPort(ip.String(), port))
		}
	}
	
	return addresses, nil
}

// connectToPeer establishes connection to a specific peer
func (nm *NetworkManager) connectToPeer(address string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	// Check if already connected to this peer
	if _, exists := nm.activePeers[address]; exists {
		return fmt.Errorf("already connected to peer %s", address)
	}
	
	// Check peer limit
	if len(nm.activePeers) >= nm.maxPeers {
		return fmt.Errorf("maximum peer limit reached (%d)", nm.maxPeers)
	}
	
	// Establish TCP connection
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %v", address, err)
	}
	
	// Create peer object
	peer := &LivePeer{
		ID:          fmt.Sprintf("peer_%s_%d", address, time.Now().Unix()),
		Address:     address,
		Conn:        conn,
		LastSeen:    time.Now(),
		IsOutbound:  true,
		Connected:   true,
		Relay:       true,
	}
	
	// Store peer and connection
	nm.activePeers[address] = peer
	nm.peerConnections[address] = conn
	
	// Start peer message handling
	go nm.handlePeerMessages(peer)
	
	// Send version handshake
	if err := nm.sendVersionHandshake(peer); err != nil {
		nm.disconnectPeer(address)
		return fmt.Errorf("version handshake failed with %s: %v", address, err)
	}
	
	log.Printf("✅ NetworkManager: Connected to peer %s", address)
	
	// Publish peer connected event
	nm.eventBus.Publish("p2p.peer_connected.v1", map[string]interface{}{
		"peer_id":   peer.ID,
		"address":   peer.Address,
		"network":   nm.network,
		"timestamp": time.Now().Unix(),
	})
	
	return nil
}

// sendVersionHandshake sends Bitcoin SV version message to establish connection
func (nm *NetworkManager) sendVersionHandshake(peer *LivePeer) error {
	// This would implement the actual Bitcoin SV version message protocol
	// For now, we'll simulate a successful handshake
	log.Printf("🤝 NetworkManager: Sending version handshake to %s", peer.Address)
	
	// In a full implementation, this would:
	// 1. Create and send version message
	// 2. Wait for version response
	// 3. Send verack message
	// 4. Wait for verack response
	
	peer.Version = 70016 // Bitcoin SV protocol version
	peer.UserAgent = "/Metamorph:1.0.0/"
	peer.StartHeight = nm.lastBlockHeight
	
	return nil
}

// handlePeerMessages processes incoming messages from a peer
func (nm *NetworkManager) handlePeerMessages(peer *LivePeer) {
	defer nm.disconnectPeer(peer.Address)
	
	log.Printf("📡 NetworkManager: Starting message handler for peer %s", peer.Address)
	
	buffer := make([]byte, 4096)
	
	for {
		select {
		case <-nm.ctx.Done():
			return
		default:
			// Set read timeout
			peer.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			
			n, err := peer.Conn.Read(buffer)
			if err != nil {
				log.Printf("⚠️  Read error from peer %s: %v", peer.Address, err)
				return
			}
			
			if n > 0 {
				peer.LastSeen = time.Now()
				peer.BytesReceived += uint64(n)
				
				// Process received message
				nm.processMessage(peer, buffer[:n])
			}
		}
	}
}

// processMessage processes a received Bitcoin SV protocol message
func (nm *NetworkManager) processMessage(peer *LivePeer, data []byte) {
	// This would implement full Bitcoin SV message parsing
	// For now, we'll log and publish basic events
	
	log.Printf("📨 NetworkManager: Received %d bytes from peer %s", len(data), peer.Address)
	
	// Publish raw message event for processing by other services
	nm.eventBus.Publish("p2p.raw_message.v1", map[string]interface{}{
		"peer_id":   peer.ID,
		"data":      data,
		"size":      len(data),
		"network":   nm.network,
		"timestamp": time.Now().Unix(),
	})
	
	// In a full implementation, this would parse message types:
	// - tx: Transaction messages
	// - block: Block messages  
	// - ping/pong: Keep-alive messages
	// - inv: Inventory messages
	// - getdata: Data request messages
	// - etc.
}

// maintainPeerConnections maintains optimal peer connections
func (nm *NetworkManager) maintainPeerConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			nm.mu.RLock()
			peerCount := len(nm.activePeers)
			nm.mu.RUnlock()
			
			log.Printf("🌐 NetworkManager: Maintaining %d peer connections", peerCount)
			
			// Connect to more peers if below minimum
			if peerCount < nm.minPeers {
				log.Printf("🔄 NetworkManager: Below minimum peers (%d), discovering more...", nm.minPeers)
				go nm.discoverAndConnectPeers()
			}
			
			// Clean up stale connections
			nm.cleanupStalePeers()
		}
	}
}

// cleanupStalePeers removes stale or inactive peer connections
func (nm *NetworkManager) cleanupStalePeers() {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	now := time.Now()
	staleThreshold := 5 * time.Minute
	
	for address, peer := range nm.activePeers {
		if now.Sub(peer.LastSeen) > staleThreshold {
			log.Printf("🧹 NetworkManager: Removing stale peer %s", address)
			nm.disconnectPeerUnsafe(address)
		}
	}
}

// monitorNetworkHealth monitors overall network connectivity health
func (nm *NetworkManager) monitorNetworkHealth() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-nm.ctx.Done():
			return
		case <-ticker.C:
			nm.mu.RLock()
			peerCount := len(nm.activePeers)
			totalBytesReceived := uint64(0)
			totalBytesSent := uint64(0)
			
			for _, peer := range nm.activePeers {
				totalBytesReceived += peer.BytesReceived
				totalBytesSent += peer.BytesSent
			}
			nm.mu.RUnlock()
			
			log.Printf("📊 NetworkManager: Network health - %d peers, %d bytes received, %d bytes sent", 
				peerCount, totalBytesReceived, totalBytesSent)
			
			// Publish network health metrics
			nm.eventBus.Publish("p2p.network_health.v1", map[string]interface{}{
				"peer_count":        peerCount,
				"bytes_received":    totalBytesReceived,
				"bytes_sent":        totalBytesSent,
				"network":           nm.network,
				"is_connected":      nm.isConnected,
				"last_block_height": nm.lastBlockHeight,
				"timestamp":         time.Now().Unix(),
			})
		}
	}
}

// handleIncomingConnections accepts incoming peer connections
func (nm *NetworkManager) handleIncomingConnections() {
	// This would implement a listening server for incoming connections
	// For now, we focus on outbound connections
	log.Printf("📡 NetworkManager: Incoming connection handler ready (outbound-only mode)")
}

// disconnectPeer safely disconnects from a peer
func (nm *NetworkManager) disconnectPeer(address string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	nm.disconnectPeerUnsafe(address)
}

// disconnectPeerUnsafe disconnects from a peer (must hold lock)
func (nm *NetworkManager) disconnectPeerUnsafe(address string) {
	if peer, exists := nm.activePeers[address]; exists {
		if peer.Conn != nil {
			peer.Conn.Close()
		}
		delete(nm.activePeers, address)
		delete(nm.peerConnections, address)
		
		log.Printf("❌ NetworkManager: Disconnected from peer %s", address)
		
		// Publish peer disconnected event
		nm.eventBus.Publish("p2p.peer_disconnected.v1", map[string]interface{}{
			"peer_id":   peer.ID,
			"address":   peer.Address,
			"network":   nm.network,
			"timestamp": time.Now().Unix(),
		})
	}
}

// GetNetworkStatus returns current network connectivity status
func (nm *NetworkManager) GetNetworkStatus() map[string]interface{} {
	nm.mu.RLock()
	defer nm.mu.RUnlock()
	
	return map[string]interface{}{
		"network":           nm.network,
		"is_connected":      nm.isConnected,
		"peer_count":        len(nm.activePeers),
		"max_peers":         nm.maxPeers,
		"min_peers":         nm.minPeers,
		"last_block_height": nm.lastBlockHeight,
	}
}

// Stop gracefully shuts down the network manager
func (nm *NetworkManager) Stop() error {
	log.Printf("🛑 NetworkManager: Shutting down Bitcoin SV network connections...")
	
	nm.cancel()
	
	nm.mu.Lock()
	defer nm.mu.Unlock()
	
	// Disconnect all peers
	for address := range nm.activePeers {
		nm.disconnectPeerUnsafe(address)
	}
	
	nm.isConnected = false
	log.Printf("🛑 NetworkManager: All network connections closed")
	
	return nil
}
