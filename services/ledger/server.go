package ledger

import (
	"context"
	"fmt"
	"log"
	"sync"

	ledgerv1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/ledger/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UTXOStore represents the interface for UTXO storage
type UTXOStore interface {
	GetOutputs(ctx context.Context, outpoints []*ledgerv1.Outpoint) ([]*ledgerv1.Output, error)
	ApplyBlock(ctx context.Context, blockHash []byte, height uint64, spends []*ledgerv1.TxSpend, outputs []*ledgerv1.NewOutput) error
	GetTipHash() []byte
	GetHeight() uint64
}

// InMemoryUTXOStore is a simple in-memory implementation for development
type InMemoryUTXOStore struct {
	mu      sync.RWMutex
	utxos   map[string]*ledgerv1.Output // key: txid:vout
	tipHash []byte
	height  uint64
}

func NewInMemoryUTXOStore() *InMemoryUTXOStore {
	return &InMemoryUTXOStore{
		utxos:   make(map[string]*ledgerv1.Output),
		tipHash: make([]byte, 32), // genesis block hash placeholder
		height:  0,
	}
}

func (s *InMemoryUTXOStore) GetOutputs(ctx context.Context, outpoints []*ledgerv1.Outpoint) ([]*ledgerv1.Output, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	outputs := make([]*ledgerv1.Output, len(outpoints))
	for i, outpoint := range outpoints {
		key := fmt.Sprintf("%x:%d", outpoint.Txid, outpoint.Vout)
		if output, exists := s.utxos[key]; exists {
			outputs[i] = output
		} else {
			// Return empty output for non-existent UTXO
			outputs[i] = &ledgerv1.Output{
				IsUnspent:     false,
				AmountSats:    0,
				ScriptPubKey:  nil,
			}
		}
	}
	return outputs, nil
}

func (s *InMemoryUTXOStore) ApplyBlock(ctx context.Context, blockHash []byte, height uint64, spends []*ledgerv1.TxSpend, outputs []*ledgerv1.NewOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove spent outputs
	for _, spend := range spends {
		key := fmt.Sprintf("%x:%d", spend.Txid, spend.Vout)
		delete(s.utxos, key)
	}

	// Add new outputs
	for _, output := range outputs {
		key := fmt.Sprintf("%x:%d", output.Txid, output.Vout)
		s.utxos[key] = &ledgerv1.Output{
			IsUnspent:     true,
			AmountSats:    output.AmountSats,
			ScriptPubKey:  output.ScriptPubKey,
		}
	}

	// Update tip
	s.tipHash = blockHash
	s.height = height

	return nil
}

func (s *InMemoryUTXOStore) GetTipHash() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tipHash
}

func (s *InMemoryUTXOStore) GetHeight() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.height
}

// Server implements the Ledger gRPC service
type Server struct {
	ledgerv1.UnimplementedLedgerServer
	store UTXOStore
}

func NewServer(store UTXOStore) *Server {
	return &Server{
		store: store,
	}
}

func (s *Server) GetOutputs(ctx context.Context, req *ledgerv1.GetOutputsRequest) (*ledgerv1.GetOutputsResponse, error) {
	if len(req.Outpoints) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no outpoints provided")
	}

	if len(req.Outpoints) > 1000 {
		return nil, status.Error(codes.InvalidArgument, "too many outpoints requested (max 1000)")
	}

	outputs, err := s.store.GetOutputs(ctx, req.Outpoints)
	if err != nil {
		log.Printf("Error getting outputs: %v", err)
		return nil, status.Error(codes.Internal, "failed to get outputs")
	}

	return &ledgerv1.GetOutputsResponse{
		Outputs:          outputs,
		WatermarkTipHash: s.store.GetTipHash(),
		Height:           s.store.GetHeight(),
	}, nil
}

func (s *Server) ApplyBlock(ctx context.Context, req *ledgerv1.ApplyBlockRequest) (*ledgerv1.ApplyBlockResponse, error) {
	if len(req.BlockHash) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid block hash length")
	}

	err := s.store.ApplyBlock(ctx, req.BlockHash, req.Height, req.Spends, req.Outputs)
	if err != nil {
		log.Printf("Error applying block: %v", err)
		return nil, status.Error(codes.Internal, "failed to apply block")
	}

	return &ledgerv1.ApplyBlockResponse{
		NewTipHash: req.BlockHash,
		Height:     req.Height,
	}, nil
}

func (s *Server) Health(ctx context.Context, req *ledgerv1.HealthRequest) (*ledgerv1.HealthResponse, error) {
	return &ledgerv1.HealthResponse{
		Status: "healthy",
	}, nil
}
