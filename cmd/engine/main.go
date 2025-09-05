package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/engine"
	enginev1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/engine/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Create script engine
	scriptEngine := engine.NewBasicScriptEngine()
	
	// Create engine server
	server := engine.NewServer(scriptEngine)
	
	// Create gRPC server
	grpcServer := grpc.NewServer()
	enginev1.RegisterEngineServer(grpcServer, server)
	
	// Enable reflection for development
	reflection.Register(grpcServer)
	
	// Listen on port 50052
	listener, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	
	log.Println("Engine service starting on :50052")
	
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
	
	log.Println("Shutting down engine service...")
	grpcServer.GracefulStop()
}
