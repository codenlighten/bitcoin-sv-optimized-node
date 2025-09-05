package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	enginev1 "github.com/codenlighten/bitcoin-sv-optimized-node/gen/engine/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ScriptEngine represents the interface for script execution
type ScriptEngine interface {
	Execute(ctx context.Context, scriptSig, scriptPubKey []byte, amount uint64, flags uint32) (*ExecutionResult, error)
}

// ExecutionResult represents the result of script execution
type ExecutionResult struct {
	Success     bool
	Error       string
	CostUnits   uint32
	DurationMs  uint32
}

// BasicScriptEngine is a placeholder implementation for development
// In production, this would integrate with a proper Bitcoin Script interpreter
type BasicScriptEngine struct{}

func NewBasicScriptEngine() *BasicScriptEngine {
	return &BasicScriptEngine{}
}

func (e *BasicScriptEngine) Execute(ctx context.Context, scriptSig, scriptPubKey []byte, amount uint64, flags uint32) (*ExecutionResult, error) {
	start := time.Now()
	
	// Placeholder script validation logic
	// In a real implementation, this would:
	// 1. Parse and validate scriptSig and scriptPubKey
	// 2. Execute the script stack operations
	// 3. Verify signatures and other constraints
	// 4. Apply resource limits (CPU, memory, opcodes)
	
	// For now, we'll do basic validation
	if len(scriptSig) == 0 && len(scriptPubKey) == 0 {
		return &ExecutionResult{
			Success:    false,
			Error:      "empty scripts",
			CostUnits:  1,
			DurationMs: uint32(time.Since(start).Milliseconds()),
		}, nil
	}
	
	// Simulate script execution cost based on script size
	costUnits := uint32(len(scriptSig) + len(scriptPubKey))
	if costUnits == 0 {
		costUnits = 1
	}
	
	// Basic validation: reject scripts that are too large
	if len(scriptSig) > 10000 || len(scriptPubKey) > 10000 {
		return &ExecutionResult{
			Success:    false,
			Error:      "script too large",
			CostUnits:  costUnits,
			DurationMs: uint32(time.Since(start).Milliseconds()),
		}, nil
	}
	
	// For development, assume most scripts are valid
	// In production, this would be much more sophisticated
	return &ExecutionResult{
		Success:    true,
		Error:      "",
		CostUnits:  costUnits,
		DurationMs: uint32(time.Since(start).Milliseconds()),
	}, nil
}

// Server implements the Engine gRPC service
type Server struct {
	enginev1.UnimplementedEngineServer
	engine ScriptEngine
}

func NewServer(engine ScriptEngine) *Server {
	return &Server{
		engine: engine,
	}
}

func (s *Server) Execute(ctx context.Context, req *enginev1.ExecuteRequest) (*enginev1.ExecuteResponse, error) {
	// Validate request
	if len(req.Txid) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid txid length")
	}
	
	if req.AmountSats == 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be greater than 0")
	}
	
	// Apply budget limits
	if req.CpuBudgetUs == 0 {
		req.CpuBudgetUs = 10000 // 10ms default
	}
	if req.OpcountBudget == 0 {
		req.OpcountBudget = 500 // reasonable default
	}
	
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(req.CpuBudgetUs)*time.Microsecond)
	defer cancel()
	
	// Execute script
	result, err := s.engine.Execute(execCtx, req.ScriptSig, req.ScriptPubKey, req.AmountSats, req.Flags)
	if err != nil {
		log.Printf("Script execution error for txid %x vin %d: %v", req.Txid, req.Vin, err)
		return &enginev1.ExecuteResponse{
			Ok:         false,
			Error:      fmt.Sprintf("execution failed: %v", err),
			CostUnits:  1,
			DurationMs: 0,
		}, nil
	}
	
	return &enginev1.ExecuteResponse{
		Ok:         result.Success,
		Error:      result.Error,
		CostUnits:  result.CostUnits,
		DurationMs: result.DurationMs,
	}, nil
}
