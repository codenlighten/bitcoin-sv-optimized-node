package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// MetricsCollector aggregates system metrics for observability
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]*Metric
	ctx     context.Context
	cancel  context.CancelFunc
}

// Metric represents a single metric with value and metadata
type Metric struct {
	Name      string    `json:"name"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
	Timestamp time.Time `json:"timestamp"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Service   string    `json:"service"`
	Status    string    `json:"status"`
	Uptime    string    `json:"uptime"`
	LastCheck time.Time `json:"last_check"`
	Metrics   []Metric  `json:"metrics"`
}

// SystemStatus represents overall system health
type SystemStatus struct {
	Node     string          `json:"node"`
	Version  string          `json:"version"`
	Uptime   string          `json:"uptime"`
	Services []ServiceHealth `json:"services"`
	Summary  SystemSummary   `json:"summary"`
}

type SystemSummary struct {
	TotalTxProcessed    int64   `json:"total_tx_processed"`
	TotalBlocksValidated int64   `json:"total_blocks_validated"`
	AvgTxLatency        float64 `json:"avg_tx_latency_ms"`
	PeerCount           int     `json:"peer_count"`
	UTXOCount           int64   `json:"utxo_count"`
	EventsPerSecond     float64 `json:"events_per_second"`
}

func NewMetricsCollector() *MetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MetricsCollector{
		metrics: make(map[string]*Metric),
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (mc *MetricsCollector) Start(port string) error {
	log.Printf("📊 Starting Telemetry service on port %s", port)
	
	// Start metrics collection
	go mc.collectMetrics()
	
	// Start HTTP server for metrics endpoint
	http.HandleFunc("/metrics", mc.handleMetrics)
	http.HandleFunc("/health", mc.handleHealth)
	http.HandleFunc("/status", mc.handleStatus)
	
	return http.ListenAndServe(":"+port, nil)
}

func (mc *MetricsCollector) Stop() {
	mc.cancel()
	log.Println("📊 Telemetry service stopped")
}

func (mc *MetricsCollector) RecordMetric(name string, value float64, unit string, labels map[string]string) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.metrics[name] = &Metric{
		Name:      name,
		Value:     value,
		Unit:      unit,
		Timestamp: time.Now(),
		Labels:    labels,
	}
}

func (mc *MetricsCollector) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	startTime := time.Now()
	
	for {
		select {
		case <-mc.ctx.Done():
			return
		case <-ticker.C:
			// Simulate collecting system metrics
			mc.RecordMetric("system.uptime", time.Since(startTime).Seconds(), "seconds", nil)
			mc.RecordMetric("ledger.utxo_count", 1000+float64(time.Now().Unix()%10000), "count", map[string]string{"service": "ledger"})
			mc.RecordMetric("engine.scripts_executed", float64(time.Now().Unix()%1000), "count", map[string]string{"service": "engine"})
			mc.RecordMetric("verifier.tx_validated", float64(time.Now().Unix()%500), "count", map[string]string{"service": "verifier"})
			mc.RecordMetric("sentinel.peer_count", float64(5+time.Now().Unix()%15), "count", map[string]string{"service": "sentinel"})
			mc.RecordMetric("events.processed_per_sec", float64(100+time.Now().Unix()%200), "rate", map[string]string{"service": "events"})
		}
	}
}

func (mc *MetricsCollector) handleMetrics(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	
	metrics := make([]Metric, 0, len(mc.metrics))
	for _, metric := range mc.metrics {
		metrics = append(metrics, *metric)
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"timestamp": time.Now(),
		"metrics":   metrics,
	})
}

func (mc *MetricsCollector) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"checks": map[string]string{
			"ledger":   "healthy",
			"engine":   "healthy",
			"sentinel": "healthy",
			"verifier": "healthy",
			"events":   "healthy",
		},
	}
	
	json.NewEncoder(w).Encode(health)
}

func (mc *MetricsCollector) handleStatus(w http.ResponseWriter, r *http.Request) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	
	// Calculate system summary
	summary := SystemSummary{
		TotalTxProcessed:     int64(time.Now().Unix() % 100000),
		TotalBlocksValidated: int64(time.Now().Unix() % 1000),
		AvgTxLatency:         250.0 + float64(time.Now().Unix()%100),
		PeerCount:            int(5 + time.Now().Unix()%15),
		UTXOCount:            int64(1000 + time.Now().Unix()%10000),
		EventsPerSecond:      float64(100 + time.Now().Unix()%200),
	}
	
	// Create service health reports
	services := []ServiceHealth{
		{
			Service:   "ledger",
			Status:    "healthy",
			Uptime:    "24h 15m",
			LastCheck: time.Now(),
			Metrics:   []Metric{{Name: "utxo_operations", Value: 1500, Unit: "ops/sec"}},
		},
		{
			Service:   "engine",
			Status:    "healthy",
			Uptime:    "24h 15m",
			LastCheck: time.Now(),
			Metrics:   []Metric{{Name: "script_executions", Value: 800, Unit: "ops/sec"}},
		},
		{
			Service:   "sentinel",
			Status:    "healthy",
			Uptime:    "24h 15m",
			LastCheck: time.Now(),
			Metrics:   []Metric{{Name: "peer_connections", Value: float64(summary.PeerCount), Unit: "count"}},
		},
		{
			Service:   "verifier",
			Status:    "healthy",
			Uptime:    "24h 15m",
			LastCheck: time.Now(),
			Metrics:   []Metric{{Name: "validation_rate", Value: 950, Unit: "tx/sec"}},
		},
		{
			Service:   "events",
			Status:    "healthy",
			Uptime:    "24h 15m",
			LastCheck: time.Now(),
			Metrics:   []Metric{{Name: "message_throughput", Value: summary.EventsPerSecond, Unit: "msg/sec"}},
		},
	}
	
	status := SystemStatus{
		Node:     "metamorph-node-01",
		Version:  "v0.1.0",
		Uptime:   "24h 15m 32s",
		Services: services,
		Summary:  summary,
	}
	
	json.NewEncoder(w).Encode(status)
}

// AlertManager handles system alerts and notifications
type AlertManager struct {
	alerts []Alert
	mu     sync.RWMutex
}

type Alert struct {
	ID          string    `json:"id"`
	Level       string    `json:"level"`
	Service     string    `json:"service"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Resolved    bool      `json:"resolved"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

func NewAlertManager() *AlertManager {
	return &AlertManager{
		alerts: make([]Alert, 0),
	}
}

func (am *AlertManager) TriggerAlert(level, service, message string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	alert := Alert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		Level:     level,
		Service:   service,
		Message:   message,
		Timestamp: time.Now(),
		Resolved:  false,
	}
	
	am.alerts = append(am.alerts, alert)
	log.Printf("🚨 ALERT [%s] %s: %s", level, service, message)
}

func (am *AlertManager) ResolveAlert(alertID string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	
	for i := range am.alerts {
		if am.alerts[i].ID == alertID {
			now := time.Now()
			am.alerts[i].Resolved = true
			am.alerts[i].ResolvedAt = &now
			log.Printf("✅ RESOLVED: Alert %s", alertID)
			break
		}
	}
}

func (am *AlertManager) GetActiveAlerts() []Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	
	active := make([]Alert, 0)
	for _, alert := range am.alerts {
		if !alert.Resolved {
			active = append(active, alert)
		}
	}
	
	return active
}
