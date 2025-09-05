package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gocql/gocql"
)

// UTXOBootstrap handles syncing the Bitcoin SV UTXO set
type UTXOBootstrap struct {
	session        *gocql.Session
	networkManager NetworkManager
	syncState      *SyncState
	workers        int
	batchSize      int
}

// SyncState tracks the bootstrap progress
type SyncState struct {
	mu              sync.RWMutex
	currentHeight   uint64
	targetHeight    uint64
	processedBlocks uint64
	utxosProcessed  uint64
	startTime       time.Time
	isActive        bool
}

// NetworkManager interface for Bitcoin SV network operations
type NetworkManager interface {
	GetBlockHeight() (uint64, error)
	GetBlockByHeight(height uint64) (*Block, error)
	GetBlockByHash(hash []byte) (*Block, error)
	GetUTXOSnapshot(height uint64) (*UTXOSnapshot, error)
}

// Block represents a Bitcoin SV block
type Block struct {
	Hash         []byte
	Height       uint64
	PrevHash     []byte
	MerkleRoot   []byte
	Timestamp    uint64
	Transactions []*Transaction
}

// Transaction represents a Bitcoin SV transaction
type Transaction struct {
	TxID    []byte
	Inputs  []*TxInput
	Outputs []*TxOutput
}

// TxInput represents a transaction input
type TxInput struct {
	PrevTxID []byte
	Vout     uint32
	Script   []byte
}

// TxOutput represents a transaction output
type TxOutput struct {
	Value  uint64
	Script []byte
}

// UTXOSnapshot represents a UTXO set snapshot
type UTXOSnapshot struct {
	Height uint64
	UTXOs  map[string]*UTXO
}

// UTXO represents an unspent transaction output
type UTXO struct {
	TxID         []byte
	Vout         uint32
	Value        uint64
	Script       []byte
	BlockHeight  uint64
	IsUnspent    bool
	CreatedAt    time.Time
}

// NewUTXOBootstrap creates a new UTXO bootstrap service
func NewUTXOBootstrap(session *gocql.Session, networkManager NetworkManager) *UTXOBootstrap {
	return &UTXOBootstrap{
		session:        session,
		networkManager: networkManager,
		syncState: &SyncState{
			startTime: time.Now(),
		},
		workers:   8,  // Parallel processing workers
		batchSize: 100, // UTXOs per batch insert
	}
}

// StartBootstrap begins the UTXO set synchronization
func (ub *UTXOBootstrap) StartBootstrap(ctx context.Context, startHeight uint64) error {
	log.Printf("🚀 Starting Bitcoin SV UTXO bootstrap from height %d", startHeight)
	
	ub.syncState.mu.Lock()
	ub.syncState.isActive = true
	ub.syncState.currentHeight = startHeight
	ub.syncState.startTime = time.Now()
	ub.syncState.mu.Unlock()

	// Get current network height
	targetHeight, err := ub.networkManager.GetBlockHeight()
	if err != nil {
		return fmt.Errorf("failed to get network height: %v", err)
	}

	ub.syncState.mu.Lock()
	ub.syncState.targetHeight = targetHeight
	ub.syncState.mu.Unlock()

	log.Printf("📊 Target height: %d, blocks to sync: %d", targetHeight, targetHeight-startHeight)

	// Try to use UTXO snapshot for faster bootstrap
	if startHeight > 0 && startHeight < targetHeight-1000 {
		log.Printf("🔄 Attempting to use UTXO snapshot for faster bootstrap...")
		if err := ub.bootstrapFromSnapshot(ctx, startHeight); err != nil {
			log.Printf("⚠️  Snapshot bootstrap failed, falling back to full sync: %v", err)
		} else {
			log.Printf("✅ Successfully bootstrapped from UTXO snapshot")
			// Continue with incremental sync from snapshot height
			startHeight = ub.syncState.currentHeight
		}
	}

	// Sync remaining blocks
	return ub.syncBlocks(ctx, startHeight, targetHeight)
}

// bootstrapFromSnapshot attempts to bootstrap from a UTXO snapshot
func (ub *UTXOBootstrap) bootstrapFromSnapshot(ctx context.Context, height uint64) error {
	log.Printf("📥 Downloading UTXO snapshot for height %d", height)
	
	snapshot, err := ub.networkManager.GetUTXOSnapshot(height)
	if err != nil {
		return fmt.Errorf("failed to get UTXO snapshot: %v", err)
	}

	log.Printf("💾 Processing %d UTXOs from snapshot", len(snapshot.UTXOs))

	// Process UTXOs in batches
	utxoBatch := make([]*UTXO, 0, ub.batchSize)
	processed := 0

	for _, utxo := range snapshot.UTXOs {
		utxoBatch = append(utxoBatch, utxo)
		
		if len(utxoBatch) >= ub.batchSize {
			if err := ub.insertUTXOBatch(ctx, utxoBatch); err != nil {
				return fmt.Errorf("failed to insert UTXO batch: %v", err)
			}
			processed += len(utxoBatch)
			utxoBatch = utxoBatch[:0]
			
			if processed%10000 == 0 {
				log.Printf("📊 Processed %d/%d UTXOs from snapshot", processed, len(snapshot.UTXOs))
			}
		}
	}

	// Insert remaining UTXOs
	if len(utxoBatch) > 0 {
		if err := ub.insertUTXOBatch(ctx, utxoBatch); err != nil {
			return fmt.Errorf("failed to insert final UTXO batch: %v", err)
		}
		processed += len(utxoBatch)
	}

	ub.syncState.mu.Lock()
	ub.syncState.currentHeight = snapshot.Height
	ub.syncState.utxosProcessed = uint64(processed)
	ub.syncState.mu.Unlock()

	log.Printf("✅ Snapshot bootstrap complete: %d UTXOs processed", processed)
	return nil
}

// syncBlocks synchronizes blocks from startHeight to targetHeight
func (ub *UTXOBootstrap) syncBlocks(ctx context.Context, startHeight, targetHeight uint64) error {
	log.Printf("🔄 Syncing blocks from %d to %d", startHeight, targetHeight)

	// Create worker channels
	blockChan := make(chan uint64, ub.workers*2)
	errChan := make(chan error, ub.workers)
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < ub.workers; i++ {
		wg.Add(1)
		go ub.blockWorker(ctx, blockChan, errChan, &wg)
	}

	// Send block heights to workers
	go func() {
		defer close(blockChan)
		for height := startHeight; height <= targetHeight; height++ {
			select {
			case blockChan <- height:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Monitor for errors
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Check for errors
	for err := range errChan {
		if err != nil {
			return fmt.Errorf("block sync error: %v", err)
		}
	}

	log.Printf("✅ Block synchronization complete")
	return nil
}

// blockWorker processes blocks in parallel
func (ub *UTXOBootstrap) blockWorker(ctx context.Context, blockChan <-chan uint64, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	for height := range blockChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := ub.processBlock(ctx, height); err != nil {
			errChan <- fmt.Errorf("failed to process block %d: %v", height, err)
			return
		}

		// Update progress
		ub.syncState.mu.Lock()
		ub.syncState.processedBlocks++
		if height > ub.syncState.currentHeight {
			ub.syncState.currentHeight = height
		}
		ub.syncState.mu.Unlock()

		// Log progress periodically
		if height%1000 == 0 {
			ub.logProgress()
		}
	}
}

// processBlock processes a single block and updates UTXO set
func (ub *UTXOBootstrap) processBlock(ctx context.Context, height uint64) error {
	block, err := ub.networkManager.GetBlockByHeight(height)
	if err != nil {
		return fmt.Errorf("failed to get block: %v", err)
	}

	// Process all transactions in the block
	for _, tx := range block.Transactions {
		// Remove spent UTXOs (transaction inputs)
		for _, input := range tx.Inputs {
			if err := ub.markUTXOSpent(ctx, input.PrevTxID, input.Vout); err != nil {
				return fmt.Errorf("failed to mark UTXO spent: %v", err)
			}
		}

		// Add new UTXOs (transaction outputs)
		for vout, output := range tx.Outputs {
			utxo := &UTXO{
				TxID:        tx.TxID,
				Vout:        uint32(vout),
				Value:       output.Value,
				Script:      output.Script,
				BlockHeight: height,
				IsUnspent:   true,
				CreatedAt:   time.Now(),
			}

			if err := ub.insertUTXO(ctx, utxo); err != nil {
				return fmt.Errorf("failed to insert UTXO: %v", err)
			}
		}
	}

	ub.syncState.mu.Lock()
	ub.syncState.utxosProcessed += uint64(len(block.Transactions))
	ub.syncState.mu.Unlock()

	return nil
}

// insertUTXO inserts a single UTXO into Scylla
func (ub *UTXOBootstrap) insertUTXO(ctx context.Context, utxo *UTXO) error {
	query := `
		INSERT INTO metamorph.utxos (txid, vout, amount_sats, script_pubkey, is_unspent, block_height, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	
	return ub.session.Query(query,
		utxo.TxID,
		utxo.Vout,
		utxo.Value,
		utxo.Script,
		utxo.IsUnspent,
		utxo.BlockHeight,
		utxo.CreatedAt,
	).WithContext(ctx).Exec()
}

// insertUTXOBatch inserts multiple UTXOs in a batch
func (ub *UTXOBootstrap) insertUTXOBatch(ctx context.Context, utxos []*UTXO) error {
	batch := ub.session.NewBatch(gocql.LoggedBatch).WithContext(ctx)
	
	query := `
		INSERT INTO metamorph.utxos (txid, vout, amount_sats, script_pubkey, is_unspent, block_height, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	for _, utxo := range utxos {
		batch.Query(query,
			utxo.TxID,
			utxo.Vout,
			utxo.Value,
			utxo.Script,
			utxo.IsUnspent,
			utxo.BlockHeight,
			utxo.CreatedAt,
		)
	}

	return ub.session.ExecuteBatch(batch)
}

// markUTXOSpent marks a UTXO as spent
func (ub *UTXOBootstrap) markUTXOSpent(ctx context.Context, txid []byte, vout uint32) error {
	query := `
		UPDATE metamorph.utxos 
		SET is_unspent = false 
		WHERE txid = ? AND vout = ?
	`
	
	return ub.session.Query(query, txid, vout).WithContext(ctx).Exec()
}

// GetSyncProgress returns the current synchronization progress
func (ub *UTXOBootstrap) GetSyncProgress() *SyncProgress {
	ub.syncState.mu.RLock()
	defer ub.syncState.mu.RUnlock()

	progress := &SyncProgress{
		CurrentHeight:   ub.syncState.currentHeight,
		TargetHeight:    ub.syncState.targetHeight,
		ProcessedBlocks: ub.syncState.processedBlocks,
		UTXOsProcessed:  ub.syncState.utxosProcessed,
		IsActive:        ub.syncState.isActive,
		StartTime:       ub.syncState.startTime,
		ElapsedTime:     time.Since(ub.syncState.startTime),
	}

	if ub.syncState.targetHeight > 0 {
		progress.PercentComplete = float64(ub.syncState.currentHeight) / float64(ub.syncState.targetHeight) * 100
	}

	if progress.ElapsedTime.Seconds() > 0 {
		progress.BlocksPerSecond = float64(ub.syncState.processedBlocks) / progress.ElapsedTime.Seconds()
		progress.UTXOsPerSecond = float64(ub.syncState.utxosProcessed) / progress.ElapsedTime.Seconds()
	}

	return progress
}

// SyncProgress represents the current sync progress
type SyncProgress struct {
	CurrentHeight    uint64        `json:"current_height"`
	TargetHeight     uint64        `json:"target_height"`
	ProcessedBlocks  uint64        `json:"processed_blocks"`
	UTXOsProcessed   uint64        `json:"utxos_processed"`
	PercentComplete  float64       `json:"percent_complete"`
	IsActive         bool          `json:"is_active"`
	StartTime        time.Time     `json:"start_time"`
	ElapsedTime      time.Duration `json:"elapsed_time"`
	BlocksPerSecond  float64       `json:"blocks_per_second"`
	UTXOsPerSecond   float64       `json:"utxos_per_second"`
}

// logProgress logs the current synchronization progress
func (ub *UTXOBootstrap) logProgress() {
	progress := ub.GetSyncProgress()
	
	log.Printf("📊 UTXO Bootstrap Progress:")
	log.Printf("   🎯 Height: %d/%d (%.1f%%)", 
		progress.CurrentHeight, progress.TargetHeight, progress.PercentComplete)
	log.Printf("   📦 Blocks: %d processed (%.1f blocks/sec)", 
		progress.ProcessedBlocks, progress.BlocksPerSecond)
	log.Printf("   💰 UTXOs: %d processed (%.1f UTXOs/sec)", 
		progress.UTXOsProcessed, progress.UTXOsPerSecond)
	log.Printf("   ⏱️  Elapsed: %v", progress.ElapsedTime.Truncate(time.Second))
	
	if progress.BlocksPerSecond > 0 {
		remaining := float64(progress.TargetHeight-progress.CurrentHeight) / progress.BlocksPerSecond
		log.Printf("   ⏳ ETA: %v", time.Duration(remaining)*time.Second)
	}
}

// Stop stops the bootstrap process
func (ub *UTXOBootstrap) Stop() {
	ub.syncState.mu.Lock()
	ub.syncState.isActive = false
	ub.syncState.mu.Unlock()
	
	log.Printf("🛑 UTXO bootstrap stopped")
}
