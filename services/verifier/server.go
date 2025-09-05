package verifier

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"time"

	"github.com/codenlighten/bitcoin-sv-optimized-node/services/events"
	eventsv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/events/v1"
	"google.golang.org/protobuf/proto"
)

// TransactionVerifier handles stateless transaction validation
type TransactionVerifier struct {
	eventPublisher *events.InMemoryPublisher
	eventLogger    *events.EventLogger
	ctx            context.Context
	cancel         context.CancelFunc
}

// Transaction represents a parsed Bitcoin transaction
type Transaction struct {
	Txid     []byte
	Version  uint32
	Inputs   []TxInput
	Outputs  []TxOutput
	LockTime uint32
	Size     uint32
}

type TxInput struct {
	PrevTxid    []byte
	PrevVout    uint32
	ScriptSig   []byte
	Sequence    uint32
	SigChecks   uint32
}

type TxOutput struct {
	Value        uint64
	ScriptPubKey []byte
}

// ValidationResult represents the outcome of transaction validation
type ValidationResult struct {
	Valid       bool
	Error       string
	SigChecks   uint32
	Size        uint32
	ProcessTime time.Duration
}

func NewTransactionVerifier(eventPublisher *events.InMemoryPublisher, network string) *TransactionVerifier {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TransactionVerifier{
		eventPublisher: eventPublisher,
		eventLogger:   events.NewEventLogger(eventPublisher, network),
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (v *TransactionVerifier) Start() {
	log.Println("Transaction Verifier starting...")
	
	// Subscribe to raw transaction events
	v.eventPublisher.Subscribe("p2p.raw_tx.v1", v.handleRawTransaction)
	
	log.Println("Transaction Verifier subscribed to p2p.raw_tx.v1 events")
}

func (v *TransactionVerifier) Stop() {
	v.cancel()
	log.Println("Transaction Verifier stopped")
}

func (v *TransactionVerifier) handleRawTransaction(ctx context.Context, event *eventsv1.Envelope) error {
	log.Printf("Verifier processing raw transaction event: %s", event.MsgId)
	
	// Parse the raw transaction event
	var rawTx eventsv1.RawTx
	if err := proto.Unmarshal(event.Payload, &rawTx); err != nil {
		log.Printf("Error unmarshaling raw tx: %v", err)
		return err
	}
	
	// Validate the transaction
	result := v.validateTransaction(rawTx.Tx)
	
	if result.Valid {
		// Create transaction ID from the raw transaction data
		txid := v.calculateTxid(rawTx.Tx)
		
		// Emit validated transaction event
		err := v.eventLogger.LogTxValidated(
			ctx,
			txid,
			result.Size,
			result.SigChecks,
			event.TraceId,
		)
		if err != nil {
			log.Printf("Error logging tx validated: %v", err)
			return err
		}
		
		log.Printf("Transaction validated successfully: %x", txid[:8])
	} else {
		log.Printf("Transaction validation failed: %s", result.Error)
		// In a real implementation, we might emit a tx.validation_failed event
	}
	
	return nil
}

func (v *TransactionVerifier) validateTransaction(txBytes []byte) *ValidationResult {
	start := time.Now()
	
	// Parse transaction (simplified for demonstration)
	tx, err := v.parseTransaction(txBytes)
	if err != nil {
		return &ValidationResult{
			Valid:       false,
			Error:       "failed to parse transaction: " + err.Error(),
			ProcessTime: time.Since(start),
		}
	}
	
	// Perform validation checks
	if err := v.performValidationChecks(tx); err != nil {
		return &ValidationResult{
			Valid:       false,
			Error:       err.Error(),
			Size:        tx.Size,
			ProcessTime: time.Since(start),
		}
	}
	
	return &ValidationResult{
		Valid:       true,
		SigChecks:   v.countSignatureChecks(tx),
		Size:        tx.Size,
		ProcessTime: time.Since(start),
	}
}

func (v *TransactionVerifier) parseTransaction(txBytes []byte) (*Transaction, error) {
	// Simplified transaction parsing for demonstration
	// In a real implementation, this would properly parse Bitcoin transaction format
	
	if len(txBytes) < 10 {
		return nil, &ValidationError{"transaction too small"}
	}
	
	// Calculate transaction ID (double SHA256)
	txid := v.calculateTxid(txBytes)
	
	// Simulate parsing - in reality this would parse the actual Bitcoin transaction format
	tx := &Transaction{
		Txid:    txid,
		Version: 1,
		Size:    uint32(len(txBytes)),
		Inputs: []TxInput{
			{
				PrevTxid:  make([]byte, 32),
				PrevVout:  0,
				ScriptSig: txBytes[10:min(50, len(txBytes))], // Simplified
				Sequence:  0xffffffff,
				SigChecks: 1,
			},
		},
		Outputs: []TxOutput{
			{
				Value:        5000000000, // 50 BSV in satoshis
				ScriptPubKey: txBytes[max(0, len(txBytes)-25):], // Simplified
			},
		},
		LockTime: 0,
	}
	
	return tx, nil
}

func (v *TransactionVerifier) performValidationChecks(tx *Transaction) error {
	// 1. Check transaction size limits
	if tx.Size > 100000000 { // 100MB max (BSV allows large transactions)
		return &ValidationError{"transaction too large"}
	}
	
	if tx.Size == 0 {
		return &ValidationError{"empty transaction"}
	}
	
	// 2. Check version
	if tx.Version == 0 {
		return &ValidationError{"invalid version"}
	}
	
	// 3. Check inputs and outputs
	if len(tx.Inputs) == 0 {
		return &ValidationError{"no inputs"}
	}
	
	if len(tx.Outputs) == 0 {
		return &ValidationError{"no outputs"}
	}
	
	// 4. Check input validity
	for i, input := range tx.Inputs {
		if len(input.PrevTxid) != 32 {
			return &ValidationError{f("input %d: invalid prev txid length", i)}
		}
		
		if len(input.ScriptSig) > 10000 {
			return &ValidationError{f("input %d: script too large", i)}
		}
	}
	
	// 5. Check output validity
	totalOutput := uint64(0)
	for i, output := range tx.Outputs {
		if output.Value > 21000000*100000000 { // Max BSV supply in satoshis
			return &ValidationError{f("output %d: value too large", i)}
		}
		
		if len(output.ScriptPubKey) > 10000 {
			return &ValidationError{f("output %d: script too large", i)}
		}
		
		totalOutput += output.Value
		if totalOutput < output.Value { // Overflow check
			return &ValidationError{"output value overflow"}
		}
	}
	
	// 6. Check signature count limits
	sigChecks := v.countSignatureChecks(tx)
	if sigChecks > 20000 { // Reasonable limit for signature operations
		return &ValidationError{"too many signature checks"}
	}
	
	return nil
}

func (v *TransactionVerifier) countSignatureChecks(tx *Transaction) uint32 {
	total := uint32(0)
	for _, input := range tx.Inputs {
		total += input.SigChecks
	}
	return total
}

func (v *TransactionVerifier) calculateTxid(txBytes []byte) []byte {
	hash1 := sha256.Sum256(txBytes)
	hash2 := sha256.Sum256(hash1[:])
	return hash2[:]
}

// ValidationError represents a transaction validation error
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func f(format string, args ...interface{}) string {
	// Simple sprintf replacement for demonstration
	return format
}
