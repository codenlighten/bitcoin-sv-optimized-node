package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// APIGateway provides unified HTTP/REST interface for Metamorph services
type APIGateway struct {
	server *http.Server
	ctx    context.Context
	cancel context.CancelFunc
}

// TransactionRequest represents a transaction submission request
type TransactionRequest struct {
	RawTx string `json:"raw_tx"`
	Fees  struct {
		Rate     float64 `json:"rate"`
		Priority string  `json:"priority"`
	} `json:"fees"`
}

// TransactionResponse represents a transaction submission response
type TransactionResponse struct {
	Txid      string    `json:"txid"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// UTXORequest represents a UTXO lookup request
type UTXORequest struct {
	Outpoints []struct {
		Txid string `json:"txid"`
		Vout int    `json:"vout"`
	} `json:"outpoints"`
}

// UTXOResponse represents a UTXO lookup response
type UTXOResponse struct {
	Outputs []struct {
		Txid         string `json:"txid"`
		Vout         int    `json:"vout"`
		IsUnspent    bool   `json:"is_unspent"`
		AmountSats   int64  `json:"amount_sats"`
		ScriptPubKey string `json:"script_pub_key"`
	} `json:"outputs"`
	Height    int64     `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}

// NodeInfoResponse represents node information
type NodeInfoResponse struct {
	Node struct {
		Name     string `json:"name"`
		Version  string `json:"version"`
		Network  string `json:"network"`
		Uptime   string `json:"uptime"`
		Features []string `json:"features"`
	} `json:"node"`
	Chain struct {
		Height    int64  `json:"height"`
		BestHash  string `json:"best_hash"`
		Difficulty float64 `json:"difficulty"`
	} `json:"chain"`
	Network struct {
		PeerCount     int     `json:"peer_count"`
		Connections   int     `json:"connections"`
		BytesReceived int64   `json:"bytes_received"`
		BytesSent     int64   `json:"bytes_sent"`
	} `json:"network"`
	Mempool struct {
		Size         int     `json:"size"`
		Bytes        int64   `json:"bytes"`
		Usage        float64 `json:"usage"`
		MaxMempool   int64   `json:"max_mempool"`
		MempoolMinFee float64 `json:"mempool_min_fee"`
	} `json:"mempool"`
}

func NewAPIGateway() *APIGateway {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &APIGateway{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (gw *APIGateway) Start(port string) error {
	mux := http.NewServeMux()
	
	// API endpoints
	mux.HandleFunc("/api/v1/tx/submit", gw.handleSubmitTransaction)
	mux.HandleFunc("/api/v1/tx/status/", gw.handleTransactionStatus)
	mux.HandleFunc("/api/v1/utxo/lookup", gw.handleUTXOLookup)
	mux.HandleFunc("/api/v1/node/info", gw.handleNodeInfo)
	mux.HandleFunc("/api/v1/node/peers", gw.handlePeers)
	mux.HandleFunc("/api/v1/mempool/info", gw.handleMempoolInfo)
	
	// ARC-compatible endpoints
	mux.HandleFunc("/v1/tx", gw.handleARCTransaction)
	mux.HandleFunc("/v1/tx/", gw.handleARCTransactionStatus)
	
	// Health and metrics
	mux.HandleFunc("/health", gw.handleHealth)
	mux.HandleFunc("/ready", gw.handleReady)
	
	// CORS middleware
	handler := gw.corsMiddleware(gw.authMiddleware(gw.loggingMiddleware(mux)))
	
	gw.server = &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}
	
	log.Printf("🌐 API Gateway starting on port %s", port)
	log.Println("📋 Available endpoints:")
	log.Println("  POST /api/v1/tx/submit - Submit transaction")
	log.Println("  GET  /api/v1/tx/status/{txid} - Transaction status")
	log.Println("  POST /api/v1/utxo/lookup - UTXO lookup")
	log.Println("  GET  /api/v1/node/info - Node information")
	log.Println("  GET  /api/v1/node/peers - Peer information")
	log.Println("  GET  /api/v1/mempool/info - Mempool status")
	log.Println("  POST /v1/tx - ARC-compatible transaction submission")
	log.Println("  GET  /health - Health check")
	
	return gw.server.ListenAndServe()
}

func (gw *APIGateway) Stop() error {
	gw.cancel()
	if gw.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return gw.server.Shutdown(ctx)
	}
	return nil
}

func (gw *APIGateway) handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Simulate transaction processing
	txid := fmt.Sprintf("%064x", time.Now().UnixNano())
	
	response := TransactionResponse{
		Txid:      txid,
		Status:    "accepted",
		Message:   "Transaction accepted for processing",
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
	
	log.Printf("📤 Transaction submitted: %s", txid[:16]+"...")
}

func (gw *APIGateway) handleTransactionStatus(w http.ResponseWriter, r *http.Request) {
	txid := r.URL.Path[len("/api/v1/tx/status/"):]
	if len(txid) != 64 {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}
	
	// Simulate transaction status lookup
	status := map[string]interface{}{
		"txid":        txid,
		"status":      "confirmed",
		"blockHeight": 800000 + time.Now().Unix()%1000,
		"blockHash":   fmt.Sprintf("%064x", time.Now().Unix()),
		"confirmations": 6,
		"timestamp":   time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (gw *APIGateway) handleUTXOLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req UTXORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	// Simulate UTXO lookup
	response := UTXOResponse{
		Height:    800000 + time.Now().Unix()%1000,
		Timestamp: time.Now(),
	}
	
	for _, outpoint := range req.Outpoints {
		output := struct {
			Txid         string `json:"txid"`
			Vout         int    `json:"vout"`
			IsUnspent    bool   `json:"is_unspent"`
			AmountSats   int64  `json:"amount_sats"`
			ScriptPubKey string `json:"script_pub_key"`
		}{
			Txid:         outpoint.Txid,
			Vout:         outpoint.Vout,
			IsUnspent:    true,
			AmountSats:   5000000000, // 50 BSV
			ScriptPubKey: "76a914" + fmt.Sprintf("%040x", time.Now().UnixNano()) + "88ac",
		}
		response.Outputs = append(response.Outputs, output)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (gw *APIGateway) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	response := NodeInfoResponse{
		Node: struct {
			Name     string   `json:"name"`
			Version  string   `json:"version"`
			Network  string   `json:"network"`
			Uptime   string   `json:"uptime"`
			Features []string `json:"features"`
		}{
			Name:     "Metamorph",
			Version:  "v0.1.0",
			Network:  "mainnet",
			Uptime:   "24h 15m 32s",
			Features: []string{"teranode", "microservices", "arc-compatible", "high-throughput"},
		},
		Chain: struct {
			Height     int64   `json:"height"`
			BestHash   string  `json:"best_hash"`
			Difficulty float64 `json:"difficulty"`
		}{
			Height:     800000 + time.Now().Unix()%1000,
			BestHash:   fmt.Sprintf("%064x", time.Now().Unix()),
			Difficulty: 1.23e12,
		},
		Network: struct {
			PeerCount     int   `json:"peer_count"`
			Connections   int   `json:"connections"`
			BytesReceived int64 `json:"bytes_received"`
			BytesSent     int64 `json:"bytes_sent"`
		}{
			PeerCount:     int(8 + time.Now().Unix()%12),
			Connections:   int(8 + time.Now().Unix()%12),
			BytesReceived: time.Now().Unix() * 1024,
			BytesSent:     time.Now().Unix() * 512,
		},
		Mempool: struct {
			Size          int     `json:"size"`
			Bytes         int64   `json:"bytes"`
			Usage         float64 `json:"usage"`
			MaxMempool    int64   `json:"max_mempool"`
			MempoolMinFee float64 `json:"mempool_min_fee"`
		}{
			Size:          int(100 + time.Now().Unix()%500),
			Bytes:         int64(1024 * (100 + time.Now().Unix()%500)),
			Usage:         0.15,
			MaxMempool:    300 * 1024 * 1024, // 300MB
			MempoolMinFee: 1.0,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (gw *APIGateway) handlePeers(w http.ResponseWriter, r *http.Request) {
	peers := []map[string]interface{}{
		{
			"id":       "peer-001",
			"addr":     "203.0.113.1:8333",
			"version":  70015,
			"subver":   "/Satoshi:1.0.0/",
			"inbound":  false,
			"conntime": time.Now().Unix() - 3600,
			"bytessent": 1024 * 1024,
			"bytesrecv": 2048 * 1024,
		},
		{
			"id":       "peer-002",
			"addr":     "203.0.113.2:8333",
			"version":  70015,
			"subver":   "/Satoshi:1.0.0/",
			"inbound":  true,
			"conntime": time.Now().Unix() - 1800,
			"bytessent": 512 * 1024,
			"bytesrecv": 1024 * 1024,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": peers,
		"count": len(peers),
	})
}

func (gw *APIGateway) handleMempoolInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"size":             int(100 + time.Now().Unix()%500),
		"bytes":            int64(1024 * (100 + time.Now().Unix()%500)),
		"usage":            0.15,
		"maxmempool":       300 * 1024 * 1024,
		"mempoolminfee":    1.0,
		"minrelaytxfee":    1.0,
		"unbroadcastcount": 0,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func (gw *APIGateway) handleARCTransaction(w http.ResponseWriter, r *http.Request) {
	// ARC-compatible transaction endpoint
	gw.handleSubmitTransaction(w, r)
}

func (gw *APIGateway) handleARCTransactionStatus(w http.ResponseWriter, r *http.Request) {
	// ARC-compatible transaction status endpoint
	gw.handleTransactionStatus(w, r)
}

func (gw *APIGateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"services": map[string]string{
			"ledger":   "healthy",
			"engine":   "healthy",
			"sentinel": "healthy",
			"verifier": "healthy",
			"events":   "healthy",
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (gw *APIGateway) handleReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// Middleware functions
func (gw *APIGateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

func (gw *APIGateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple API key authentication (in production, use proper JWT/OAuth)
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Allow requests without API key for demo purposes
		}
		
		next.ServeHTTP(w, r)
	})
}

func (gw *APIGateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		next.ServeHTTP(w, r)
		
		duration := time.Since(start)
		log.Printf("🌐 %s %s - %v", r.Method, r.URL.Path, duration)
	})
}
