package miner

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// MinerServer integrates Bitcoin SV mining functionality with Metamorph architecture
type MinerServer struct {
	mu           sync.RWMutex
	network      string
	miningAddr   string
	eventBus     EventBus
	miningService *MiningService
	stratumClient *StratumClient
	ctx          context.Context
	cancel       context.CancelFunc
	isActive     bool
	poolMode     bool
	poolURL      string
	poolUsername string
	poolPassword string
	startTime    time.Time
}

// NewMinerServer creates a new Bitcoin SV mining server
func NewMinerServer(network string, eventBus EventBus) *MinerServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Get mining configuration from environment
	miningAddr := os.Getenv("BITCOIN_MINING_ADDRESS")
	if miningAddr == "" {
		miningAddr = "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa" // Default address (Genesis block)
	}
	
	// Check if pool mining is configured
	poolURL := os.Getenv("BITCOIN_POOL_URL")
	poolUsername := os.Getenv("BITCOIN_POOL_USERNAME")
	poolPassword := os.Getenv("BITCOIN_POOL_PASSWORD")
	
	poolMode := poolURL != "" && poolUsername != ""
	
	server := &MinerServer{
		network:      network,
		miningAddr:   miningAddr,
		eventBus:     eventBus,
		ctx:          ctx,
		cancel:       cancel,
		poolMode:     poolMode,
		poolURL:      poolURL,
		poolUsername: poolUsername,
		poolPassword: poolPassword,
		startTime:    time.Now(),
	}
	
	// Initialize mining service
	server.miningService = NewMiningService(network, miningAddr, eventBus)
	
	// Initialize Stratum client if pool mode is enabled
	if poolMode {
		server.stratumClient = NewStratumClient(poolURL, poolUsername, poolPassword, eventBus)
	}
	
	return server
}

// Start begins the Bitcoin SV mining server
func (ms *MinerServer) Start() error {
	mode := "Solo Mining"
	if ms.poolMode {
		mode = "Pool Mining"
	}
	
	log.Printf("⛏️  Miner Server: Starting Bitcoin SV %s (%s)...", ms.network, mode)
	
	// Start mining service
	if err := ms.miningService.Start(); err != nil {
		return fmt.Errorf("failed to start mining service: %v", err)
	}
	
	// Connect to mining pool if configured
	if ms.poolMode {
		if err := ms.connectToPool(); err != nil {
			log.Printf("⚠️  Warning: Failed to connect to mining pool: %v", err)
			log.Printf("⛏️  Falling back to solo mining mode...")
			ms.poolMode = false
		}
	}
	
	// Start mining management routines
	go ms.manageMining()
	go ms.monitorPerformance()
	
	ms.mu.Lock()
	ms.isActive = true
	ms.mu.Unlock()
	
	log.Printf("⛏️  Miner Server: Bitcoin SV mining started successfully")
	
	// Publish mining server started event
	ms.eventBus.Publish("miner.server_started.v1", map[string]interface{}{
		"network":     ms.network,
		"mode":        mode,
		"pool_url":    ms.poolURL,
		"mining_addr": ms.miningAddr,
		"timestamp":   time.Now().Unix(),
	})
	
	return nil
}

// connectToPool establishes connection to mining pool
func (ms *MinerServer) connectToPool() error {
	if ms.stratumClient == nil {
		return fmt.Errorf("stratum client not initialized")
	}
	
	log.Printf("⛏️  Connecting to mining pool: %s", ms.poolURL)
	
	if err := ms.stratumClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to pool: %v", err)
	}
	
	// Wait for authorization
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			return fmt.Errorf("pool authorization timeout")
		case <-ticker.C:
			status := ms.stratumClient.GetPoolStatus()
			if authorized, ok := status["authorized"].(bool); ok && authorized {
				log.Printf("⛏️  Successfully connected and authorized with mining pool")
				return nil
			}
		}
	}
}

// manageMining coordinates mining operations
func (ms *MinerServer) manageMining() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ms.ctx.Done():
			return
		case <-ticker.C:
			ms.coordinateMining()
		}
	}
}

// coordinateMining coordinates between solo mining and pool mining
func (ms *MinerServer) coordinateMining() {
	ms.mu.RLock()
	poolMode := ms.poolMode
	ms.mu.RUnlock()
	
	if poolMode {
		// Pool mining coordination
		ms.handlePoolMining()
	} else {
		// Solo mining coordination
		ms.handleSoloMining()
	}
}

// handlePoolMining manages pool mining operations
func (ms *MinerServer) handlePoolMining() {
	if ms.stratumClient == nil {
		return
	}
	
	poolStatus := ms.stratumClient.GetPoolStatus()
	
	// Check if we have a current job
	if jobID, ok := poolStatus["current_job"].(string); ok && jobID != "" {
		// We have a mining job from the pool
		// In a full implementation, this would:
		// 1. Convert Stratum job to block template
		// 2. Distribute work to mining workers
		// 3. Submit shares back to pool when found
		
		log.Printf("⛏️  Pool Mining: Working on job %s", jobID)
	}
}

// handleSoloMining manages solo mining operations
func (ms *MinerServer) handleSoloMining() {
	// Solo mining uses the mining service directly
	// The mining service creates its own block templates
	// and attempts to find complete block solutions
	
	status := ms.miningService.GetMiningStatus()
	if isActive, ok := status["is_active"].(bool); ok && isActive {
		// Mining is active, check for any blocks found
		if blocksFound, ok := status["blocks_found"].(uint64); ok && blocksFound > 0 {
			log.Printf("⛏️  Solo Mining: %d blocks found", blocksFound)
		}
	}
}

// monitorPerformance monitors mining performance and reports metrics
func (ms *MinerServer) monitorPerformance() {
	ticker := time.NewTicker(60 * time.Second) // Report every minute
	defer ticker.Stop()
	
	for {
		select {
		case <-ms.ctx.Done():
			return
		case <-ticker.C:
			ms.reportPerformance()
		}
	}
}

// reportPerformance reports current mining performance metrics
func (ms *MinerServer) reportPerformance() {
	ms.mu.RLock()
	isActive := ms.isActive
	poolMode := ms.poolMode
	startTime := ms.startTime
	ms.mu.RUnlock()
	
	if !isActive {
		return
	}
	
	// Get mining service status
	miningStatus := ms.miningService.GetMiningStatus()
	
	hashRate := uint64(0)
	if hr, ok := miningStatus["hash_rate"].(uint64); ok {
		hashRate = hr
	}
	
	activeWorkers := 0
	if aw, ok := miningStatus["active_workers"].(int); ok {
		activeWorkers = aw
	}
	
	blocksFound := uint64(0)
	if bf, ok := miningStatus["blocks_found"].(uint64); ok {
		blocksFound = bf
	}
	
	uptime := time.Since(startTime)
	
	// Get pool status if in pool mode
	var poolStatus map[string]interface{}
	if poolMode && ms.stratumClient != nil {
		poolStatus = ms.stratumClient.GetPoolStatus()
	}
	
	log.Printf("⛏️  Mining Performance Report:")
	log.Printf("   📊 Hash Rate: %d H/s", hashRate)
	log.Printf("   👷 Active Workers: %d", activeWorkers)
	log.Printf("   🎯 Blocks Found: %d", blocksFound)
	log.Printf("   ⏱️  Uptime: %v", uptime.Truncate(time.Second))
	
	if poolMode && poolStatus != nil {
		if connected, ok := poolStatus["connected"].(bool); ok && connected {
			sharesSubmitted := uint64(0)
			if ss, ok := poolStatus["shares_submitted"].(uint64); ok {
				sharesSubmitted = ss
			}
			
			difficulty := float64(0)
			if d, ok := poolStatus["difficulty"].(float64); ok {
				difficulty = d
			}
			
			log.Printf("   🏊 Pool Connected: %s", ms.poolURL)
			log.Printf("   📤 Shares Submitted: %d", sharesSubmitted)
			log.Printf("   🎯 Pool Difficulty: %.6f", difficulty)
		} else {
			log.Printf("   ⚠️  Pool Disconnected: %s", ms.poolURL)
		}
	} else {
		log.Printf("   🏠 Solo Mining Mode")
	}
	
	// Publish performance metrics event
	performanceData := map[string]interface{}{
		"hash_rate":      hashRate,
		"active_workers": activeWorkers,
		"blocks_found":   blocksFound,
		"uptime_seconds": int64(uptime.Seconds()),
		"network":        ms.network,
		"mode":           "solo",
		"timestamp":      time.Now().Unix(),
	}
	
	if poolMode {
		performanceData["mode"] = "pool"
		if poolStatus != nil {
			performanceData["pool_status"] = poolStatus
		}
	}
	
	ms.eventBus.Publish("miner.performance.v1", performanceData)
}

// GetMiningStatus returns comprehensive mining status
func (ms *MinerServer) GetMiningStatus() map[string]interface{} {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	
	status := map[string]interface{}{
		"is_active":     ms.isActive,
		"network":       ms.network,
		"pool_mode":     ms.poolMode,
		"mining_addr":   ms.miningAddr,
		"uptime":        time.Since(ms.startTime).Seconds(),
		"start_time":    ms.startTime.Unix(),
	}
	
	// Add mining service status
	if ms.miningService != nil {
		miningStatus := ms.miningService.GetMiningStatus()
		for k, v := range miningStatus {
			status["mining_"+k] = v
		}
	}
	
	// Add pool status if available
	if ms.poolMode && ms.stratumClient != nil {
		poolStatus := ms.stratumClient.GetPoolStatus()
		status["pool"] = poolStatus
	}
	
	return status
}

// SetMiningConfiguration updates mining configuration
func (ms *MinerServer) SetMiningConfiguration(config map[string]interface{}) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	// Update mining address
	if addr, ok := config["mining_address"].(string); ok && addr != "" {
		ms.miningAddr = addr
		log.Printf("⛏️  Updated mining address: %s", addr)
	}
	
	// Update worker count
	if workers, ok := config["max_workers"].(int); ok && workers > 0 {
		if ms.miningService != nil {
			ms.miningService.maxWorkers = workers
			log.Printf("⛏️  Updated max workers: %d", workers)
		}
	}
	
	// Update pool configuration
	if poolURL, ok := config["pool_url"].(string); ok {
		ms.poolURL = poolURL
		ms.poolMode = poolURL != ""
		
		if ms.poolMode {
			if username, ok := config["pool_username"].(string); ok {
				ms.poolUsername = username
			}
			if password, ok := config["pool_password"].(string); ok {
				ms.poolPassword = password
			}
			
			// Reconnect to new pool
			if ms.stratumClient != nil {
				ms.stratumClient.Disconnect()
			}
			
			ms.stratumClient = NewStratumClient(ms.poolURL, ms.poolUsername, ms.poolPassword, ms.eventBus)
			
			if ms.isActive {
				go func() {
					if err := ms.connectToPool(); err != nil {
						log.Printf("⚠️  Failed to connect to new pool: %v", err)
					}
				}()
			}
		}
	}
	
	return nil
}

// Stop gracefully shuts down the mining server
func (ms *MinerServer) Stop() error {
	log.Printf("🛑 Miner Server: Shutting down Bitcoin SV mining...")
	
	ms.cancel()
	
	ms.mu.Lock()
	defer ms.mu.Unlock()
	
	// Stop mining service
	if ms.miningService != nil {
		if err := ms.miningService.Stop(); err != nil {
			log.Printf("⚠️  Error stopping mining service: %v", err)
		}
	}
	
	// Disconnect from mining pool
	if ms.stratumClient != nil {
		if err := ms.stratumClient.Disconnect(); err != nil {
			log.Printf("⚠️  Error disconnecting from pool: %v", err)
		}
	}
	
	ms.isActive = false
	
	uptime := time.Since(ms.startTime)
	log.Printf("🛑 Miner Server: Bitcoin SV mining stopped (uptime: %v)", uptime.Truncate(time.Second))
	
	// Publish mining server stopped event
	ms.eventBus.Publish("miner.server_stopped.v1", map[string]interface{}{
		"network":        ms.network,
		"uptime_seconds": int64(uptime.Seconds()),
		"timestamp":      time.Now().Unix(),
	})
	
	return nil
}
