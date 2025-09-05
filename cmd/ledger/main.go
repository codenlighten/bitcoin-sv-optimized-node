package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/ledger"
	ledgerv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/ledger/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Create UTXO store (in-memory for development)
	store := ledger.NewInMemoryUTXOStore()
	
	// Create ledger server
	server := ledger.NewServer(store)
	
	// Create gRPC server
	grpcServer := grpc.NewServer()
	ledgerv1.RegisterLedgerServer(grpcServer, server)
	
	// Enable reflection for development
	reflection.Register(grpcServer)
	
	// Listen on port 50051
	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	
	log.Println("Ledger service starting on :50051")
	
	// Start server in goroutine
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()
	
	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	
	log.Println("Shutting down ledger service...")
	grpcServer.GracefulStop()
}
