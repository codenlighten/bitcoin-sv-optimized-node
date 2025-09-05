package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/events"
	"github.com/codenlighten/bitcoin-sv-optimized-node/services/ledger"
	"github.com/codenlighten/bitcoin-sv-optimized-node/services/engine"
	"github.com/codenlighten/bitcoin-sv-optimized-node/services/sentinel"
	"github.com/codenlighten/bitcoin-sv-optimized-node/services/verifier"
	ledgerv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/ledger/v1"
	enginev1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/engine/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
)

// MetamorphNode represents the complete Metamorph Bitcoin SV node
type MetamorphNode struct {
	// Event system
	eventPublisher *events.InMemoryPublisher
	
	// Core services
	ledgerService    *ledger.Server
	engineService    *engine.Server
	sentinelService  *sentinel.P2PServer
	verifierService  *verifier.TransactionVerifier
	
	// gRPC servers
	ledgerGRPC *grpc.Server
	engineGRPC *grpc.Server
	
	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewMetamorphNode() *MetamorphNode {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create event publisher
	eventPublisher := events.NewInMemoryPublisher()
	
	// Create UTXO store
	utxoStore := ledger.NewInMemoryUTXOStore()
	
	// Create script engine
	scriptEngine := engine.NewBasicScriptEngine()
	
	// Create services
	ledgerService := ledger.NewServer(utxoStore)
	engineService := engine.NewServer(scriptEngine)
	sentinelService := sentinel.NewP2PServer(eventPublisher, "main")
	verifierService := verifier.NewTransactionVerifier(eventPublisher, "main")
	
	return &MetamorphNode{
		eventPublisher:  eventPublisher,
		ledgerService:   ledgerService,
		engineService:   engineService,
		sentinelService: sentinelService,
		verifierService: verifierService,
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (n *MetamorphNode) Start() error {
	log.Println("🚀 Starting Metamorph Bitcoin SV Node...")
	
	// Start event system
	log.Println("📡 Starting event system...")
	
	// Start verifier service
	log.Println("✅ Starting transaction verifier...")
	n.verifierService.Start()
	
	// Start Ledger gRPC service
	log.Println("💾 Starting Ledger gRPC service on :50051...")
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := n.startLedgerGRPC(); err != nil {
			log.Printf("Ledger gRPC error: %v", err)
		}
	}()
	
	// Start Engine gRPC service
	log.Println("⚙️  Starting Engine gRPC service on :50052...")
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := n.startEngineGRPC(); err != nil {
			log.Printf("Engine gRPC error: %v", err)
		}
	}()
	
	// Start Sentinel P2P service
	log.Println("🌐 Starting Sentinel P2P service on :8333...")
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		if err := n.sentinelService.Start("8333"); err != nil {
			log.Printf("Sentinel P2P error: %v", err)
		}
	}()
	
	// Start status reporter
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.reportStatus()
	}()
	
	log.Println("🎉 Metamorph node started successfully!")
	log.Println("")
	log.Println("📋 Service endpoints:")
	log.Println("  - Ledger gRPC:  localhost:50051")
	log.Println("  - Engine gRPC:  localhost:50052")
	log.Println("  - P2P Network:  localhost:8333")
	log.Println("")
	log.Println("🔄 Event flows active:")
	log.Println("  - p2p.raw_tx.v1 → tx.validated.v1")
	log.Println("  - p2p.raw_block.v1 → block.validated.v1")
	log.Println("  - tx.validated.v1 → tx.ready.v1")
	log.Println("")
	
	return nil
}

func (n *MetamorphNode) startLedgerGRPC() error {
	n.ledgerGRPC = grpc.NewServer()
	ledgerv1.RegisterLedgerServer(n.ledgerGRPC, n.ledgerService)
	reflection.Register(n.ledgerGRPC)
	
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		return err
	}
	
	return n.ledgerGRPC.Serve(listener)
}

func (n *MetamorphNode) startEngineGRPC() error {
	n.engineGRPC = grpc.NewServer()
	enginev1.RegisterEngineServer(n.engineGRPC, n.engineService)
	reflection.Register(n.engineGRPC)
	
	listener, err := net.Listen("tcp", ":50052")
	if err != nil {
		return err
	}
	
	return n.engineGRPC.Serve(listener)
}

func (n *MetamorphNode) reportStatus() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			peerCount := n.sentinelService.GetPeerCount()
			log.Printf("📊 Node Status - Peers: %d, Services: Healthy", peerCount)
		}
	}
}

func (n *MetamorphNode) Stop() {
	log.Println("🛑 Shutting down Metamorph node...")
	
	// Cancel context to signal all services to stop
	n.cancel()
	
	// Stop gRPC servers gracefully
	if n.ledgerGRPC != nil {
		log.Println("💾 Stopping Ledger gRPC...")
		n.ledgerGRPC.GracefulStop()
	}
	
	if n.engineGRPC != nil {
		log.Println("⚙️  Stopping Engine gRPC...")
		n.engineGRPC.GracefulStop()
	}
	
	// Stop other services
	log.Println("🌐 Stopping Sentinel P2P...")
	n.sentinelService.Stop()
	
	log.Println("✅ Stopping Verifier...")
	n.verifierService.Stop()
	
	log.Println("📡 Stopping event system...")
	n.eventPublisher.Close()
	
	// Wait for all goroutines to finish
	n.wg.Wait()
	
	log.Println("✨ Metamorph node stopped gracefully")
}

func main() {
	// Create and start the Metamorph node
	node := NewMetamorphNode()
	
	// Start the node
	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start Metamorph node: %v", err)
	}
	
	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	
	// Graceful shutdown
	node.Stop()
}
