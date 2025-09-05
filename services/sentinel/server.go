package sentinel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net"
	"sync"
	"time"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/events"
	eventsv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/events/v1"
)

// Peer represents a connected Bitcoin SV peer
type Peer struct {
	ID       string
	Conn     net.Conn
	Address  string
	Version  uint32
	Services uint64
	LastSeen time.Time
}

// P2PServer handles Bitcoin SV peer-to-peer networking
type P2PServer struct {
	mu            sync.RWMutex
	peers         map[string]*Peer
	eventPublisher *events.InMemoryPublisher
	eventLogger   *events.EventLogger
	listener      net.Listener
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewP2PServer(eventPublisher *events.InMemoryPublisher, network string) *P2PServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &P2PServer{
		peers:         make(map[string]*Peer),
		eventPublisher: eventPublisher,
		eventLogger:   events.NewEventLogger(eventPublisher, network),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (s *P2PServer) Start(port string) error {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	
	s.listener = listener
	log.Printf("Sentinel P2P server listening on port %s", port)
	
	// Start accepting connections
	go s.acceptConnections()
	
	// Start peer maintenance
	go s.maintainPeers()
	
	// Simulate receiving transactions for demonstration
	go s.simulateIncomingTransactions()
	
	return nil
}

func (s *P2PServer) Stop() error {
	s.cancel()
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *P2PServer) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.listener.Accept()
			if err != nil {
				if s.ctx.Err() != nil {
					return // Server is shutting down
				}
				log.Printf("Error accepting connection: %v", err)
				continue
			}
			
			go s.handlePeer(conn)
		}
	}
}

func (s *P2PServer) handlePeer(conn net.Conn) {
	defer conn.Close()
	
	// Generate peer ID
	peerID := generatePeerID()
	
	peer := &Peer{
		ID:       peerID,
		Conn:     conn,
		Address:  conn.RemoteAddr().String(),
		LastSeen: time.Now(),
	}
	
	s.mu.Lock()
	s.peers[peerID] = peer
	s.mu.Unlock()
	
	log.Printf("New peer connected: %s (%s)", peerID, peer.Address)
	
	// Handle peer messages (simplified for demonstration)
	buffer := make([]byte, 4096)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				log.Printf("Peer %s disconnected: %v", peerID, err)
				s.removePeer(peerID)
				return
			}
			
			// Process received data (simplified)
			s.processMessage(peer, buffer[:n])
		}
	}
}

func (s *P2PServer) processMessage(peer *Peer, data []byte) {
	// In a real implementation, this would parse Bitcoin protocol messages
	// For demonstration, we'll simulate different message types
	
	peer.LastSeen = time.Now()
	
	if len(data) < 4 {
		return
	}
	
	// Simulate transaction message detection
	if data[0] == 0x01 { // Simulated "tx" message type
		s.handleTransactionMessage(peer, data[4:])
	} else if data[0] == 0x02 { // Simulated "block" message type
		s.handleBlockMessage(peer, data[4:])
	}
}

func (s *P2PServer) handleTransactionMessage(peer *Peer, txData []byte) {
	log.Printf("Received transaction from peer %s, size: %d bytes", peer.ID, len(txData))
	
	// Create raw transaction event
	builder := events.NewEventBuilder("main")
	rawTx, err := builder.CreateRawTxEvent(txData, peer.ID)
	if err != nil {
		log.Printf("Error creating raw tx event: %v", err)
		return
	}
	
	envelope, err := builder.CreateEnvelope("trace-"+generateTraceID(), rawTx)
	if err != nil {
		log.Printf("Error creating envelope: %v", err)
		return
	}
	
	// Publish to event bus
	err = s.eventPublisher.Publish(context.Background(), "p2p.raw_tx.v1", envelope)
	if err != nil {
		log.Printf("Error publishing raw tx event: %v", err)
	}
}

func (s *P2PServer) handleBlockMessage(peer *Peer, blockData []byte) {
	log.Printf("Received block from peer %s, size: %d bytes", peer.ID, len(blockData))
	
	// Create raw block event
	builder := events.NewEventBuilder("main")
	rawBlock := &eventsv1.RawBlock{
		Block:  blockData,
		PeerId: peer.ID,
		SeeAt:  time.Now().UTC().Format(time.RFC3339),
	}
	
	envelope, err := builder.CreateEnvelope("trace-"+generateTraceID(), rawBlock)
	if err != nil {
		log.Printf("Error creating envelope: %v", err)
		return
	}
	
	// Publish to event bus
	err = s.eventPublisher.Publish(context.Background(), "p2p.raw_block.v1", envelope)
	if err != nil {
		log.Printf("Error publishing raw block event: %v", err)
	}
}

func (s *P2PServer) maintainPeers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupStaleConnections()
		}
	}
}

func (s *P2PServer) cleanupStaleConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	cutoff := time.Now().Add(-5 * time.Minute)
	for peerID, peer := range s.peers {
		if peer.LastSeen.Before(cutoff) {
			log.Printf("Removing stale peer: %s", peerID)
			peer.Conn.Close()
			delete(s.peers, peerID)
		}
	}
}

func (s *P2PServer) removePeer(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.peers, peerID)
}

func (s *P2PServer) GetPeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

func (s *P2PServer) simulateIncomingTransactions() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	txCounter := 0
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Simulate receiving a transaction from a peer
			txCounter++
			
			// Create fake transaction data
			txData := make([]byte, 250) // Typical transaction size
			rand.Read(txData)
			
			// Simulate peer
			fakePeer := &Peer{
				ID:      "sim-peer-" + hex.EncodeToString(txData[:4]),
				Address: "127.0.0.1:8333",
			}
			
			s.handleTransactionMessage(fakePeer, txData)
			log.Printf("Simulated incoming transaction #%d", txCounter)
		}
	}
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
