package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/events"
	eventsv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/events/v1"
)

// Conductor orchestrates transaction processing
type Conductor struct {
	eventPublisher *events.InMemoryPublisher
	eventLogger    *events.EventLogger
}

func NewConductor() *Conductor {
	publisher := events.NewInMemoryPublisher()
	logger := events.NewEventLogger(publisher, "main")
	
	return &Conductor{
		eventPublisher: publisher,
		eventLogger:    logger,
	}
}

func (c *Conductor) Start(ctx context.Context) {
	log.Println("Conductor service starting...")
	
	// Subscribe to raw transaction events
	c.eventPublisher.Subscribe("p2p.raw_tx.v1", c.handleRawTx)
	
	// Subscribe to validated transaction events
	c.eventPublisher.Subscribe("tx.validated.v1", c.handleTxValidated)
	
	// Simulate some transaction processing for demonstration
	go c.simulateTransactionFlow(ctx)
	
	<-ctx.Done()
	log.Println("Conductor service stopping...")
}

func (c *Conductor) handleRawTx(ctx context.Context, event *eventsv1.Envelope) error {
	log.Printf("Received raw tx event: %s", event.MsgId)
	
	// In a real implementation, this would:
	// 1. Parse the transaction
	// 2. Perform basic validation
	// 3. Send to Verifier service
	// 4. Track transaction state
	
	return nil
}

func (c *Conductor) handleTxValidated(ctx context.Context, event *eventsv1.Envelope) error {
	log.Printf("Received tx validated event: %s", event.MsgId)
	
	// In a real implementation, this would:
	// 1. Update transaction state
	// 2. Check dependencies
	// 3. Calculate fees and priority
	// 4. Emit tx.ready event when ready
	
	return nil
}

func (c *Conductor) simulateTransactionFlow(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	txCounter := 0
	for {
		select {
		case <-ticker.C:
			txCounter++
			
			// Simulate a transaction becoming ready
			txid := make([]byte, 32)
			for i := range txid {
				txid[i] = byte(txCounter % 256)
			}
			
			err := c.eventLogger.LogTxReady(
				ctx,
				txid,
				1.5, // fee rate
				2,   // ancestors
				250, // weight
				"normal",
				"trace-"+string(rune(txCounter)),
			)
			if err != nil {
				log.Printf("Error logging tx ready: %v", err)
			} else {
				log.Printf("Simulated tx ready: %x", txid[:8])
			}
			
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	conductor := NewConductor()
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start conductor in goroutine
	go conductor.Start(ctx)
	
	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	
	log.Println("Shutting down conductor service...")
	cancel()
	conductor.eventPublisher.Close()
}
