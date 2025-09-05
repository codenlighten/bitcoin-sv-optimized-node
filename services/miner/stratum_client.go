package miner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// StratumClient implements Bitcoin SV mining pool connectivity via Stratum protocol
type StratumClient struct {
	mu           sync.RWMutex
	poolURL      string
	username     string
	password     string
	conn         net.Conn
	connected    bool
	subscribed   bool
	authorized   bool
	jobID        string
	extraNonce1  string
	extraNonce2Size int
	difficulty   float64
	target       string
	ctx          context.Context
	cancel       context.CancelFunc
	eventBus     EventBus
	lastJob      time.Time
	sharesSubmitted uint64
	sharesAccepted  uint64
	sharesRejected  uint64
}

// StratumMessage represents a Stratum protocol message
type StratumMessage struct {
	ID     interface{} `json:"id"`
	Method string      `json:"method,omitempty"`
	Params interface{} `json:"params,omitempty"`
	Result interface{} `json:"result,omitempty"`
	Error  interface{} `json:"error,omitempty"`
}

// StratumJob represents a mining job from the pool
type StratumJob struct {
	JobID          string   `json:"job_id"`
	PrevHash       string   `json:"prevhash"`
	CoinB1         string   `json:"coinb1"`
	CoinB2         string   `json:"coinb2"`
	MerkleBranches []string `json:"merkle_branch"`
	Version        string   `json:"version"`
	NBits          string   `json:"nbits"`
	NTime          string   `json:"ntime"`
	CleanJobs      bool     `json:"clean_jobs"`
}

// NewStratumClient creates a new Stratum mining pool client
func NewStratumClient(poolURL, username, password string, eventBus EventBus) *StratumClient {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &StratumClient{
		poolURL:  poolURL,
		username: username,
		password: password,
		eventBus: eventBus,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Connect establishes connection to the mining pool
func (sc *StratumClient) Connect() error {
	log.Printf("🏊 Stratum: Connecting to mining pool %s...", sc.poolURL)
	
	// Parse pool URL and establish TCP connection
	conn, err := net.DialTimeout("tcp", sc.poolURL, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to pool: %v", err)
	}
	
	sc.mu.Lock()
	sc.conn = conn
	sc.connected = true
	sc.mu.Unlock()
	
	// Start message handling
	go sc.handleMessages()
	
	// Subscribe to mining notifications
	if err := sc.subscribe(); err != nil {
		return fmt.Errorf("failed to subscribe: %v", err)
	}
	
	// Authorize worker
	if err := sc.authorize(); err != nil {
		return fmt.Errorf("failed to authorize: %v", err)
	}
	
	log.Printf("🏊 Stratum: Successfully connected to mining pool")
	
	// Publish pool connected event
	sc.eventBus.Publish("mining.pool_connected.v1", map[string]interface{}{
		"pool_url":  sc.poolURL,
		"username":  sc.username,
		"timestamp": time.Now().Unix(),
	})
	
	return nil
}

// subscribe sends mining.subscribe request to the pool
func (sc *StratumClient) subscribe() error {
	msg := StratumMessage{
		ID:     1,
		Method: "mining.subscribe",
		Params: []interface{}{"Metamorph/1.0"},
	}
	
	if err := sc.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send subscribe: %v", err)
	}
	
	log.Printf("🏊 Stratum: Sent mining.subscribe request")
	return nil
}

// authorize sends mining.authorize request to the pool
func (sc *StratumClient) authorize() error {
	msg := StratumMessage{
		ID:     2,
		Method: "mining.authorize",
		Params: []interface{}{sc.username, sc.password},
	}
	
	if err := sc.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to send authorize: %v", err)
	}
	
	log.Printf("🏊 Stratum: Sent mining.authorize request for user %s", sc.username)
	return nil
}

// sendMessage sends a Stratum message to the pool
func (sc *StratumClient) sendMessage(msg StratumMessage) error {
	sc.mu.RLock()
	conn := sc.conn
	sc.mu.RUnlock()
	
	if conn == nil {
		return fmt.Errorf("not connected to pool")
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}
	
	data = append(data, '\n')
	
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}
	
	return nil
}

// handleMessages processes incoming messages from the mining pool
func (sc *StratumClient) handleMessages() {
	scanner := bufio.NewScanner(sc.conn)
	
	for scanner.Scan() {
		select {
		case <-sc.ctx.Done():
			return
		default:
			line := scanner.Text()
			if err := sc.processMessage(line); err != nil {
				log.Printf("⚠️  Stratum: Error processing message: %v", err)
			}
		}
	}
	
	if err := scanner.Err(); err != nil {
		log.Printf("⚠️  Stratum: Scanner error: %v", err)
	}
	
	log.Printf("🏊 Stratum: Message handler stopped")
}

// processMessage processes a single Stratum message
func (sc *StratumClient) processMessage(line string) error {
	var msg StratumMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %v", err)
	}
	
	// Handle different message types
	switch {
	case msg.Method == "mining.notify":
		return sc.handleMiningNotify(msg)
	case msg.Method == "mining.set_difficulty":
		return sc.handleSetDifficulty(msg)
	case msg.ID == 1: // Subscribe response
		return sc.handleSubscribeResponse(msg)
	case msg.ID == 2: // Authorize response
		return sc.handleAuthorizeResponse(msg)
	default:
		log.Printf("🏊 Stratum: Received message: %s", line)
	}
	
	return nil
}

// handleSubscribeResponse processes mining.subscribe response
func (sc *StratumClient) handleSubscribeResponse(msg StratumMessage) error {
	if msg.Error != nil {
		return fmt.Errorf("subscribe failed: %v", msg.Error)
	}
	
	result, ok := msg.Result.([]interface{})
	if !ok || len(result) < 3 {
		return fmt.Errorf("invalid subscribe response format")
	}
	
	// Extract subscription details
	if extraNonce1, ok := result[1].(string); ok {
		sc.mu.Lock()
		sc.extraNonce1 = extraNonce1
		sc.mu.Unlock()
	}
	
	if extraNonce2Size, ok := result[2].(float64); ok {
		sc.mu.Lock()
		sc.extraNonce2Size = int(extraNonce2Size)
		sc.subscribed = true
		sc.mu.Unlock()
	}
	
	log.Printf("🏊 Stratum: Successfully subscribed - ExtraNonce1: %s, ExtraNonce2Size: %d", 
		sc.extraNonce1, sc.extraNonce2Size)
	
	return nil
}

// handleAuthorizeResponse processes mining.authorize response
func (sc *StratumClient) handleAuthorizeResponse(msg StratumMessage) error {
	if msg.Error != nil {
		return fmt.Errorf("authorization failed: %v", msg.Error)
	}
	
	if result, ok := msg.Result.(bool); ok && result {
		sc.mu.Lock()
		sc.authorized = true
		sc.mu.Unlock()
		
		log.Printf("🏊 Stratum: Successfully authorized worker %s", sc.username)
		
		// Publish authorization success event
		sc.eventBus.Publish("mining.pool_authorized.v1", map[string]interface{}{
			"pool_url":  sc.poolURL,
			"username":  sc.username,
			"timestamp": time.Now().Unix(),
		})
		
		return nil
	}
	
	return fmt.Errorf("authorization denied")
}

// handleMiningNotify processes mining.notify (new job) messages
func (sc *StratumClient) handleMiningNotify(msg StratumMessage) error {
	params, ok := msg.Params.([]interface{})
	if !ok || len(params) < 9 {
		return fmt.Errorf("invalid mining.notify format")
	}
	
	job := StratumJob{}
	
	// Parse job parameters
	if jobID, ok := params[0].(string); ok {
		job.JobID = jobID
	}
	if prevHash, ok := params[1].(string); ok {
		job.PrevHash = prevHash
	}
	if coinB1, ok := params[2].(string); ok {
		job.CoinB1 = coinB1
	}
	if coinB2, ok := params[3].(string); ok {
		job.CoinB2 = coinB2
	}
	if version, ok := params[5].(string); ok {
		job.Version = version
	}
	if nBits, ok := params[6].(string); ok {
		job.NBits = nBits
	}
	if nTime, ok := params[7].(string); ok {
		job.NTime = nTime
	}
	if cleanJobs, ok := params[8].(bool); ok {
		job.CleanJobs = cleanJobs
	}
	
	sc.mu.Lock()
	sc.jobID = job.JobID
	sc.lastJob = time.Now()
	sc.mu.Unlock()
	
	log.Printf("🏊 Stratum: Received new mining job %s (clean: %v)", job.JobID, job.CleanJobs)
	
	// Publish new job event
	sc.eventBus.Publish("mining.job_received.v1", map[string]interface{}{
		"job_id":     job.JobID,
		"prev_hash":  job.PrevHash,
		"clean_jobs": job.CleanJobs,
		"timestamp":  time.Now().Unix(),
	})
	
	return nil
}

// handleSetDifficulty processes mining.set_difficulty messages
func (sc *StratumClient) handleSetDifficulty(msg StratumMessage) error {
	params, ok := msg.Params.([]interface{})
	if !ok || len(params) < 1 {
		return fmt.Errorf("invalid set_difficulty format")
	}
	
	if difficulty, ok := params[0].(float64); ok {
		sc.mu.Lock()
		sc.difficulty = difficulty
		sc.mu.Unlock()
		
		log.Printf("🏊 Stratum: Pool difficulty set to %.6f", difficulty)
		
		// Publish difficulty change event
		sc.eventBus.Publish("mining.difficulty_changed.v1", map[string]interface{}{
			"difficulty": difficulty,
			"timestamp":  time.Now().Unix(),
		})
	}
	
	return nil
}

// SubmitShare submits a mining share to the pool
func (sc *StratumClient) SubmitShare(jobID, extraNonce2, nTime, nonce string) error {
	sc.mu.Lock()
	username := sc.username
	sc.sharesSubmitted++
	shareCount := sc.sharesSubmitted
	sc.mu.Unlock()
	
	msg := StratumMessage{
		ID:     shareCount + 100, // Unique ID for share submission
		Method: "mining.submit",
		Params: []interface{}{username, jobID, extraNonce2, nTime, nonce},
	}
	
	if err := sc.sendMessage(msg); err != nil {
		return fmt.Errorf("failed to submit share: %v", err)
	}
	
	log.Printf("🏊 Stratum: Submitted share #%d for job %s (nonce: %s)", shareCount, jobID, nonce)
	
	// Publish share submitted event
	sc.eventBus.Publish("mining.share_submitted.v1", map[string]interface{}{
		"job_id":      jobID,
		"nonce":       nonce,
		"share_count": shareCount,
		"timestamp":   time.Now().Unix(),
	})
	
	return nil
}

// GetPoolStatus returns current mining pool connection status
func (sc *StratumClient) GetPoolStatus() map[string]interface{} {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	
	return map[string]interface{}{
		"pool_url":         sc.poolURL,
		"connected":        sc.connected,
		"subscribed":       sc.subscribed,
		"authorized":       sc.authorized,
		"current_job":      sc.jobID,
		"difficulty":       sc.difficulty,
		"shares_submitted": sc.sharesSubmitted,
		"shares_accepted":  sc.sharesAccepted,
		"shares_rejected":  sc.sharesRejected,
		"last_job":         sc.lastJob.Unix(),
	}
}

// Disconnect closes the connection to the mining pool
func (sc *StratumClient) Disconnect() error {
	log.Printf("🏊 Stratum: Disconnecting from mining pool...")
	
	sc.cancel()
	
	sc.mu.Lock()
	defer sc.mu.Unlock()
	
	if sc.conn != nil {
		sc.conn.Close()
		sc.conn = nil
	}
	
	sc.connected = false
	sc.subscribed = false
	sc.authorized = false
	
	log.Printf("🏊 Stratum: Disconnected from mining pool")
	
	// Publish pool disconnected event
	sc.eventBus.Publish("mining.pool_disconnected.v1", map[string]interface{}{
		"pool_url":  sc.poolURL,
		"timestamp": time.Now().Unix(),
	})
	
	return nil
}
