package sentinel

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// Bitcoin SV Protocol Implementation
// Replaces the simulation with real Bitcoin P2P networking

const (
	// Bitcoin SV Protocol Constants
	ProtocolVersion = 70016
	MainNetMagic    = 0xe3e1f3e8 // Bitcoin SV mainnet magic bytes
	TestNetMagic    = 0xf4e5f3f4 // Bitcoin SV testnet magic bytes
	
	// Message Types
	MsgVersion     = "version"
	MsgVerAck      = "verack"
	MsgPing        = "ping"
	MsgPong        = "pong"
	MsgGetAddr     = "getaddr"
	MsgAddr        = "addr"
	MsgInv         = "inv"
	MsgGetData     = "getdata"
	MsgTx          = "tx"
	MsgBlock       = "block"
	MsgGetBlocks   = "getblocks"
	MsgGetHeaders  = "getheaders"
	MsgHeaders     = "headers"
	MsgMempool     = "mempool"
	
	// Service Flags
	NodeNetwork = 1 << 0 // NODE_NETWORK
	NodeBloom   = 1 << 2 // NODE_BLOOM
	NodeWitness = 1 << 3 // NODE_WITNESS
	
	MaxMessageSize = 32 * 1024 * 1024 // 32MB max message size for Bitcoin SV
)

// BitcoinProtocol handles real Bitcoin SV P2P protocol
type BitcoinProtocol struct {
	network     string
	magic       uint32
	version     int32
	services    uint64
	userAgent   string
	startHeight int32
	relay       bool
	peers       map[string]*Peer
	eventBus    EventBus
}

// Peer represents a Bitcoin SV network peer
type Peer struct {
	conn        net.Conn
	addr        string
	version     int32
	services    uint64
	userAgent   string
	startHeight int32
	lastSeen    time.Time
	connected   bool
}

// Message represents a Bitcoin protocol message
type Message struct {
	Magic    uint32
	Command  [12]byte
	Length   uint32
	Checksum [4]byte
	Payload  []byte
}

// NewBitcoinProtocol creates a new Bitcoin SV protocol handler
func NewBitcoinProtocol(network string, eventBus EventBus) *BitcoinProtocol {
	var magic uint32
	if network == "mainnet" {
		magic = MainNetMagic
	} else {
		magic = TestNetMagic
	}
	
	return &BitcoinProtocol{
		network:     network,
		magic:       magic,
		version:     ProtocolVersion,
		services:    NodeNetwork | NodeBloom,
		userAgent:   "/Metamorph:1.0.0/",
		startHeight: 0, // Will be updated during sync
		relay:       true,
		peers:       make(map[string]*Peer),
		eventBus:    eventBus,
	}
}

// ConnectToPeers establishes connections to Bitcoin SV network peers
func (bp *BitcoinProtocol) ConnectToPeers() error {
	// Bitcoin SV DNS seeds for peer discovery
	var dnsSeeds []string
	if bp.network == "mainnet" {
		dnsSeeds = []string{
			"seed.bitcoinsv.io",
			"seed.cascharia.com",
			"seed.satoshisvision.network",
		}
	} else {
		dnsSeeds = []string{
			"testnet-seed.bitcoinsv.io",
			"testnet-seed.cascharia.com",
		}
	}
	
	// Discover peers from DNS seeds
	peerAddrs, err := bp.discoverPeers(dnsSeeds)
	if err != nil {
		return fmt.Errorf("peer discovery failed: %v", err)
	}
	
	// Connect to discovered peers
	for i, addr := range peerAddrs {
		if i >= 8 { // Limit initial connections
			break
		}
		go bp.connectToPeer(addr)
	}
	
	return nil
}

// discoverPeers discovers peer addresses from DNS seeds
func (bp *BitcoinProtocol) discoverPeers(dnsSeeds []string) ([]string, error) {
	var peerAddrs []string
	
	for _, seed := range dnsSeeds {
		addrs, err := net.LookupHost(seed)
		if err != nil {
			continue
		}
		
		for _, addr := range addrs {
			peerAddr := fmt.Sprintf("%s:8333", addr)
			peerAddrs = append(peerAddrs, peerAddr)
		}
	}
	
	if len(peerAddrs) == 0 {
		return nil, fmt.Errorf("no peers discovered")
	}
	
	return peerAddrs, nil
}

// connectToPeer establishes connection to a single peer
func (bp *BitcoinProtocol) connectToPeer(addr string) {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return
	}
	
	peer := &Peer{
		conn:      conn,
		addr:      addr,
		lastSeen:  time.Now(),
		connected: true,
	}
	
	bp.peers[addr] = peer
	
	// Send version message to initiate handshake
	if err := bp.sendVersion(peer); err != nil {
		bp.disconnectPeer(peer)
		return
	}
	
	// Start message handling loop
	go bp.handlePeer(peer)
}

// sendVersion sends a version message to initiate handshake
func (bp *BitcoinProtocol) sendVersion(peer *Peer) error {
	nonce := make([]byte, 8)
	rand.Read(nonce)
	
	// Build version message payload
	var payload bytes.Buffer
	
	// Version
	binary.Write(&payload, binary.LittleEndian, bp.version)
	
	// Services
	binary.Write(&payload, binary.LittleEndian, bp.services)
	
	// Timestamp
	binary.Write(&payload, binary.LittleEndian, time.Now().Unix())
	
	// Addr_recv (recipient address)
	bp.writeNetAddr(&payload, bp.services, peer.addr)
	
	// Addr_from (sender address)
	bp.writeNetAddr(&payload, bp.services, "0.0.0.0:8333")
	
	// Nonce
	payload.Write(nonce)
	
	// User agent
	bp.writeVarString(&payload, bp.userAgent)
	
	// Start height
	binary.Write(&payload, binary.LittleEndian, bp.startHeight)
	
	// Relay
	if bp.relay {
		payload.WriteByte(1)
	} else {
		payload.WriteByte(0)
	}
	
	return bp.sendMessage(peer, MsgVersion, payload.Bytes())
}

// handlePeer handles incoming messages from a peer
func (bp *BitcoinProtocol) handlePeer(peer *Peer) {
	defer bp.disconnectPeer(peer)
	
	for peer.connected {
		msg, err := bp.readMessage(peer)
		if err != nil {
			return
		}
		
		if err := bp.processMessage(peer, msg); err != nil {
			return
		}
		
		peer.lastSeen = time.Now()
	}
}

// readMessage reads a Bitcoin protocol message from peer
func (bp *BitcoinProtocol) readMessage(peer *Peer) (*Message, error) {
	// Read message header (24 bytes)
	header := make([]byte, 24)
	if _, err := io.ReadFull(peer.conn, header); err != nil {
		return nil, err
	}
	
	// Parse header
	msg := &Message{}
	buf := bytes.NewReader(header)
	
	binary.Read(buf, binary.LittleEndian, &msg.Magic)
	binary.Read(buf, binary.LittleEndian, &msg.Command)
	binary.Read(buf, binary.LittleEndian, &msg.Length)
	binary.Read(buf, binary.LittleEndian, &msg.Checksum)
	
	// Verify magic bytes
	if msg.Magic != bp.magic {
		return nil, fmt.Errorf("invalid magic bytes")
	}
	
	// Read payload if present
	if msg.Length > 0 {
		if msg.Length > MaxMessageSize {
			return nil, fmt.Errorf("message too large: %d bytes", msg.Length)
		}
		
		msg.Payload = make([]byte, msg.Length)
		if _, err := io.ReadFull(peer.conn, msg.Payload); err != nil {
			return nil, err
		}
		
		// Verify checksum
		hash := sha256.Sum256(msg.Payload)
		hash = sha256.Sum256(hash[:])
		if !bytes.Equal(hash[:4], msg.Checksum[:]) {
			return nil, fmt.Errorf("invalid checksum")
		}
	}
	
	return msg, nil
}

// processMessage processes an incoming message from peer
func (bp *BitcoinProtocol) processMessage(peer *Peer, msg *Message) error {
	command := string(bytes.TrimRight(msg.Command[:], "\x00"))
	
	switch command {
	case MsgVersion:
		return bp.handleVersion(peer, msg.Payload)
	case MsgVerAck:
		return bp.handleVerAck(peer)
	case MsgPing:
		return bp.handlePing(peer, msg.Payload)
	case MsgPong:
		return bp.handlePong(peer, msg.Payload)
	case MsgAddr:
		return bp.handleAddr(peer, msg.Payload)
	case MsgInv:
		return bp.handleInv(peer, msg.Payload)
	case MsgTx:
		return bp.handleTx(peer, msg.Payload)
	case MsgBlock:
		return bp.handleBlock(peer, msg.Payload)
	case MsgHeaders:
		return bp.handleHeaders(peer, msg.Payload)
	default:
		// Unknown message type, ignore
		return nil
	}
}

// handleVersion processes version message
func (bp *BitcoinProtocol) handleVersion(peer *Peer, payload []byte) error {
	buf := bytes.NewReader(payload)
	
	// Parse version message
	binary.Read(buf, binary.LittleEndian, &peer.version)
	binary.Read(buf, binary.LittleEndian, &peer.services)
	
	// Skip timestamp and addresses
	buf.Seek(8+26+26+8, io.SeekCurrent)
	
	// Read user agent
	userAgent, err := bp.readVarString(buf)
	if err != nil {
		return err
	}
	peer.userAgent = userAgent
	
	// Read start height
	binary.Read(buf, binary.LittleEndian, &peer.startHeight)
	
	// Send verack response
	return bp.sendMessage(peer, MsgVerAck, nil)
}

// handleVerAck processes verack message
func (bp *BitcoinProtocol) handleVerAck(peer *Peer) error {
	// Handshake complete, peer is now ready
	bp.eventBus.Publish("p2p.peer_connected.v1", map[string]interface{}{
		"peer_addr":    peer.addr,
		"version":      peer.version,
		"user_agent":   peer.userAgent,
		"start_height": peer.startHeight,
	})
	
	// Request peer addresses
	return bp.sendMessage(peer, MsgGetAddr, nil)
}

// handleTx processes transaction message
func (bp *BitcoinProtocol) handleTx(peer *Peer, payload []byte) error {
	// Publish raw transaction to event bus for processing
	bp.eventBus.Publish("p2p.raw_tx.v1", map[string]interface{}{
		"peer_addr": peer.addr,
		"tx_data":   payload,
		"timestamp": time.Now().Unix(),
	})
	
	return nil
}

// handleBlock processes block message
func (bp *BitcoinProtocol) handleBlock(peer *Peer, payload []byte) error {
	// Publish raw block to event bus for processing
	bp.eventBus.Publish("p2p.raw_block.v1", map[string]interface{}{
		"peer_addr":  peer.addr,
		"block_data": payload,
		"timestamp":  time.Now().Unix(),
	})
	
	return nil
}

// sendMessage sends a message to peer
func (bp *BitcoinProtocol) sendMessage(peer *Peer, command string, payload []byte) error {
	// Build message
	var msg bytes.Buffer
	
	// Magic
	binary.Write(&msg, binary.LittleEndian, bp.magic)
	
	// Command (12 bytes, null-padded)
	cmd := make([]byte, 12)
	copy(cmd, command)
	msg.Write(cmd)
	
	// Length
	binary.Write(&msg, binary.LittleEndian, uint32(len(payload)))
	
	// Checksum
	if len(payload) > 0 {
		hash := sha256.Sum256(payload)
		hash = sha256.Sum256(hash[:])
		msg.Write(hash[:4])
	} else {
		msg.Write(make([]byte, 4))
	}
	
	// Payload
	if len(payload) > 0 {
		msg.Write(payload)
	}
	
	_, err := peer.conn.Write(msg.Bytes())
	return err
}

// Helper functions for protocol message construction
func (bp *BitcoinProtocol) writeNetAddr(w io.Writer, services uint64, addr string) {
	binary.Write(w, binary.LittleEndian, services)
	
	// IPv6 address (16 bytes) - IPv4 mapped
	ip := make([]byte, 16)
	ip[10] = 0xff
	ip[11] = 0xff
	
	host, port, _ := net.SplitHostPort(addr)
	ipAddr := net.ParseIP(host)
	if ipAddr != nil {
		if ipv4 := ipAddr.To4(); ipv4 != nil {
			copy(ip[12:], ipv4)
		}
	}
	w.Write(ip)
	
	// Port (big endian)
	if port != "" {
		if p, err := net.LookupPort("tcp", port); err == nil {
			binary.Write(w, binary.BigEndian, uint16(p))
		} else {
			binary.Write(w, binary.BigEndian, uint16(8333))
		}
	} else {
		binary.Write(w, binary.BigEndian, uint16(8333))
	}
}

func (bp *BitcoinProtocol) writeVarString(w io.Writer, s string) {
	bp.writeVarInt(w, uint64(len(s)))
	w.Write([]byte(s))
}

func (bp *BitcoinProtocol) writeVarInt(w io.Writer, i uint64) {
	if i < 0xfd {
		w.Write([]byte{byte(i)})
	} else if i <= 0xffff {
		w.Write([]byte{0xfd})
		binary.Write(w, binary.LittleEndian, uint16(i))
	} else if i <= 0xffffffff {
		w.Write([]byte{0xfe})
		binary.Write(w, binary.LittleEndian, uint32(i))
	} else {
		w.Write([]byte{0xff})
		binary.Write(w, binary.LittleEndian, uint64(i))
	}
}

func (bp *BitcoinProtocol) readVarString(r io.Reader) (string, error) {
	length, err := bp.readVarInt(r)
	if err != nil {
		return "", err
	}
	
	if length > MaxMessageSize {
		return "", fmt.Errorf("string too long")
	}
	
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", err
	}
	
	return string(data), nil
}

func (bp *BitcoinProtocol) readVarInt(r io.Reader) (uint64, error) {
	var b [1]byte
	if _, err := r.Read(b[:]); err != nil {
		return 0, err
	}
	
	switch b[0] {
	case 0xfd:
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), err
	case 0xfe:
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), err
	case 0xff:
		var v uint64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	default:
		return uint64(b[0]), nil
	}
}

// Additional message handlers (simplified for now)
func (bp *BitcoinProtocol) handlePing(peer *Peer, payload []byte) error {
	return bp.sendMessage(peer, MsgPong, payload)
}

func (bp *BitcoinProtocol) handlePong(peer *Peer, payload []byte) error {
	return nil // Pong received, connection is alive
}

func (bp *BitcoinProtocol) handleAddr(peer *Peer, payload []byte) error {
	// TODO: Parse and store peer addresses for future connections
	return nil
}

func (bp *BitcoinProtocol) handleInv(peer *Peer, payload []byte) error {
	// TODO: Process inventory messages (new transactions/blocks available)
	return nil
}

func (bp *BitcoinProtocol) handleHeaders(peer *Peer, payload []byte) error {
	// TODO: Process block headers for blockchain synchronization
	return nil
}

func (bp *BitcoinProtocol) disconnectPeer(peer *Peer) {
	peer.connected = false
	peer.conn.Close()
	delete(bp.peers, peer.addr)
	
	bp.eventBus.Publish("p2p.peer_disconnected.v1", map[string]interface{}{
		"peer_addr": peer.addr,
		"timestamp": time.Now().Unix(),
	})
}
