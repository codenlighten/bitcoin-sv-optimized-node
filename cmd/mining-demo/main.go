package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/miner"
)

// SimpleMiningDemo demonstrates Bitcoin SV mining functionality
type SimpleMiningDemo struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	minerServer *miner.MinerServer
	eventBus   *SimpleEventBus
}

// SimpleEventBus implements a basic event bus for the demo
type SimpleEventBus struct {
	mu        sync.RWMutex
	listeners map[string][]func(interface{})
}

func NewSimpleEventBus() *SimpleEventBus {
	return &SimpleEventBus{
		listeners: make(map[string][]func(interface{})),
	}
}

func (eb *SimpleEventBus) Publish(topic string, data interface{}) error {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	if listeners, ok := eb.listeners[topic]; ok {
		for _, listener := range listeners {
			go listener(data)
		}
	}
	
	// Log the event for demo purposes
	log.Printf("📡 Event: %s -> %+v", topic, data)
	return nil
}

func (eb *SimpleEventBus) Subscribe(topic string, listener func(interface{})) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	eb.listeners[topic] = append(eb.listeners[topic], listener)
}

func NewSimpleMiningDemo() *SimpleMiningDemo {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Get network configuration
	network := os.Getenv("BITCOIN_NETWORK")
	if network == "" {
		network = "testnet"
	}
	
	eventBus := NewSimpleEventBus()
	minerServer := miner.NewMinerServer(network, eventBus)
	
	return &SimpleMiningDemo{
		ctx:         ctx,
		cancel:      cancel,
		minerServer: minerServer,
		eventBus:    eventBus,
	}
}

func (d *SimpleMiningDemo) Start() {
	liveMode := os.Getenv("BITCOIN_LIVE_MODE") == "true"
	network := os.Getenv("BITCOIN_NETWORK")
	if network == "" {
		network = "testnet"
	}
	
	mode := "Demo Mode"
	if liveMode {
		mode = "LIVE Mode"
	}
	
	log.Println("🚀 Starting Metamorph Bitcoin SV Mining Demo...")
	log.Printf("⚙️  Configuration: %s on Bitcoin SV %s", mode, network)
	log.Println("")
	
	// Subscribe to mining events
	d.subscribeToMiningEvents()
	
	// Start mining server
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.runMiningServer()
	}()
	
	// Start monitoring
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.monitorMining()
	}()
	
	log.Println("🎉 Bitcoin SV Mining Demo started successfully!")
	log.Println("")
	log.Println("📋 Mining Services:")
	log.Println("  ⛏️  Mining Service - Block template creation and proof-of-work")
	log.Println("  🏊 Stratum Client  - Mining pool connectivity (if configured)")
	log.Println("  📊 Performance Monitor - Hash rate and mining statistics")
	log.Println("")
	
	if liveMode {
		log.Println("🌐 LIVE MODE: Connecting to real Bitcoin SV network for mining")
		log.Println("💡 Pool Configuration:")
		log.Printf("   BITCOIN_POOL_URL: %s", os.Getenv("BITCOIN_POOL_URL"))
		log.Printf("   BITCOIN_POOL_USERNAME: %s", os.Getenv("BITCOIN_POOL_USERNAME"))
	} else {
		log.Println("🎭 DEMO MODE: Simulating Bitcoin SV mining operations")
		log.Println("💡 To enable live mining, set BITCOIN_LIVE_MODE=true")
	}
	log.Println("")
}

func (d *SimpleMiningDemo) subscribeToMiningEvents() {
	// Subscribe to mining events for logging
	d.eventBus.Subscribe("mining.started.v1", func(data interface{}) {
		log.Printf("🎯 Mining Started: %+v", data)
	})
	
	d.eventBus.Subscribe("mining.block_found.v1", func(data interface{}) {
		log.Printf("🎉 Block Found: %+v", data)
	})
	
	d.eventBus.Subscribe("mining.progress.v1", func(data interface{}) {
		if progressData, ok := data.(map[string]interface{}); ok {
			hashRate := progressData["hash_rate"]
			workers := progressData["active_workers"]
			blocks := progressData["blocks_found"]
			log.Printf("📊 Mining Progress: %d workers, %v H/s, %v blocks", workers, hashRate, blocks)
		}
	})
	
	d.eventBus.Subscribe("mining.pool_connected.v1", func(data interface{}) {
		log.Printf("🏊 Pool Connected: %+v", data)
	})
	
	d.eventBus.Subscribe("mining.share_submitted.v1", func(data interface{}) {
		log.Printf("📤 Share Submitted: %+v", data)
	})
}

func (d *SimpleMiningDemo) runMiningServer() {
	log.Println("⛏️  Starting Bitcoin SV Mining Server...")
	
	if err := d.minerServer.Start(); err != nil {
		log.Printf("❌ Failed to start mining server: %v", err)
		return
	}
	
	// Keep mining server running
	<-d.ctx.Done()
	
	log.Println("⛏️  Stopping Bitcoin SV Mining Server...")
	if err := d.minerServer.Stop(); err != nil {
		log.Printf("⚠️  Error stopping mining server: %v", err)
	}
}

func (d *SimpleMiningDemo) monitorMining() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			status := d.minerServer.GetMiningStatus()
			uptime := time.Since(startTime)
			
			log.Println("📊 Mining Status Report:")
			log.Printf("   ⏱️  Uptime: %v", uptime.Truncate(time.Second))
			
			if isActive, ok := status["is_active"].(bool); ok && isActive {
				if hashRate, ok := status["mining_hash_rate"].(uint64); ok {
					log.Printf("   🔥 Hash Rate: %d H/s", hashRate)
				}
				
				if workers, ok := status["mining_active_workers"].(int); ok {
					log.Printf("   👷 Active Workers: %d", workers)
				}
				
				if blocks, ok := status["mining_blocks_found"].(uint64); ok {
					log.Printf("   🎯 Blocks Found: %d", blocks)
				}
				
				if poolMode, ok := status["pool_mode"].(bool); ok && poolMode {
					if poolData, ok := status["pool"].(map[string]interface{}); ok {
						if connected, ok := poolData["connected"].(bool); ok {
							log.Printf("   🏊 Pool Status: %v", map[bool]string{true: "Connected", false: "Disconnected"}[connected])
						}
						
						if shares, ok := poolData["shares_submitted"].(uint64); ok {
							log.Printf("   📤 Shares Submitted: %d", shares)
						}
					}
				} else {
					log.Printf("   🏠 Mining Mode: Solo")
				}
			} else {
				log.Printf("   ⚠️  Status: Inactive")
			}
			log.Println("")
		}
	}
}

func (d *SimpleMiningDemo) Stop() {
	log.Println("🛑 Stopping Bitcoin SV Mining Demo...")
	d.cancel()
	d.wg.Wait()
	log.Println("✨ Mining Demo stopped gracefully")
}

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                METAMORPH BITCOIN SV MINING DEMO             ║")
	fmt.Println("║            Real Bitcoin SV Mining Implementation            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println("")
	
	demo := NewSimpleMiningDemo()
	demo.Start()
	
	// Wait for interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	
	demo.Stop()
	
	fmt.Println("")
	fmt.Println("🎯 Bitcoin SV Mining Demo completed!")
	fmt.Println("")
	fmt.Println("✅ Implemented Features:")
	fmt.Println("   • Real Bitcoin SV block template creation")
	fmt.Println("   • Proof-of-work mining with configurable difficulty")
	fmt.Println("   • Multi-threaded mining workers with hash rate monitoring")
	fmt.Println("   • Stratum mining pool protocol support")
	fmt.Println("   • Live network connectivity for real mining operations")
	fmt.Println("   • Comprehensive mining statistics and performance metrics")
	fmt.Println("")
	fmt.Println("🚀 Next Steps:")
	fmt.Println("   • Configure mining pool credentials for live mining")
	fmt.Println("   • Deploy to production with BITCOIN_LIVE_MODE=true")
	fmt.Println("   • Scale mining workers based on hardware capabilities")
	fmt.Println("   • Integrate with Bitcoin SV ecosystem services")
	fmt.Println("")
	fmt.Println("📚 Configuration:")
	fmt.Println("   • BITCOIN_LIVE_MODE=true     - Enable live Bitcoin SV mining")
	fmt.Println("   • BITCOIN_NETWORK=testnet    - Select network (testnet/mainnet)")
	fmt.Println("   • BITCOIN_POOL_URL=...       - Mining pool Stratum URL")
	fmt.Println("   • BITCOIN_POOL_USERNAME=...  - Mining pool username")
	fmt.Println("   • BITCOIN_MINING_ADDRESS=... - Bitcoin SV address for rewards")
}
