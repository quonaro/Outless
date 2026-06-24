package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	"outless/internal/domain"
)

// SystemMetricsBody is the flat JSON payload for system metrics.
type SystemMetricsBody struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryPercent    float64 `json:"memory_percent"`
	MemoryUsedBytes  uint64  `json:"memory_used_bytes"`
	MemoryTotalBytes uint64  `json:"memory_total_bytes"`
	NetRXBytesPerSec float64 `json:"net_rx_bytes_per_sec"`
	NetTXBytesPerSec float64 `json:"net_tx_bytes_per_sec"`
	ConnectionsCount int     `json:"connections_count"`
}

// SystemMetricsOutput is the Huma response wrapper.
type SystemMetricsOutput struct {
	Body SystemMetricsBody
}

// SystemMetricsHandler serves live system metrics.
type SystemMetricsHandler struct {
	runtime     domain.RuntimeController
	logger      *slog.Logger
	mu          sync.Mutex
	lastNetRX   uint64
	lastNetTX   uint64
	lastNetTime time.Time

	cached   SystemMetricsBody
	cachedMu sync.RWMutex
	stopCh   chan struct{}
}

// NewSystemMetricsHandler creates a system metrics handler and starts background collection.
func NewSystemMetricsHandler(runtime domain.RuntimeController, logger *slog.Logger) *SystemMetricsHandler {
	h := &SystemMetricsHandler{
		runtime: runtime,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
	go h.collectLoop()
	return h
}

// Stop halts the background metrics collector.
func (h *SystemMetricsHandler) Stop() {
	close(h.stopCh)
}

// latest returns the most recently collected metrics snapshot.
func (h *SystemMetricsHandler) latest() SystemMetricsBody {
	h.cachedMu.RLock()
	defer h.cachedMu.RUnlock()
	return h.cached
}

func (h *SystemMetricsHandler) collectLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.collectOnce()
		case <-h.stopCh:
			return
		}
	}
}

func (h *SystemMetricsHandler) collectOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var body SystemMetricsBody

	cpuPercents, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err != nil {
		h.logger.Warn("failed to get cpu percent", slog.String("error", err.Error()))
	} else if len(cpuPercents) > 0 {
		body.CPUPercent = cpuPercents[0]
	}

	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		h.logger.Warn("failed to get memory stats", slog.String("error", err.Error()))
	} else if vmStat != nil {
		body.MemoryPercent = vmStat.UsedPercent
		body.MemoryUsedBytes = vmStat.Used
		body.MemoryTotalBytes = vmStat.Total
	}

	now := time.Now()
	netIO, err := net.IOCountersWithContext(ctx, false)
	if err != nil {
		h.logger.Warn("failed to get net io counters", slog.String("error", err.Error()))
	} else if len(netIO) > 0 {
		totalRX := netIO[0].BytesRecv
		totalTX := netIO[0].BytesSent

		h.mu.Lock()
		elapsed := now.Sub(h.lastNetTime).Seconds()
		if h.lastNetTime.IsZero() || elapsed <= 0 {
			body.NetRXBytesPerSec = 0
			body.NetTXBytesPerSec = 0
		} else {
			body.NetRXBytesPerSec = float64(totalRX-h.lastNetRX) / elapsed
			body.NetTXBytesPerSec = float64(totalTX-h.lastNetTX) / elapsed
		}
		h.lastNetRX = totalRX
		h.lastNetTX = totalTX
		h.lastNetTime = now
		h.mu.Unlock()
	}

	snap := h.runtime.TrafficSnapshot()
	if snap != nil {
		uniqueUsers := make(map[string]struct{}, len(snap.Connections))
		for _, c := range snap.Connections {
			if c.User != "" {
				uniqueUsers[c.User] = struct{}{}
			}
		}
		body.ConnectionsCount = len(uniqueUsers)
	}

	h.cachedMu.Lock()
	h.cached = body
	h.cachedMu.Unlock()
}

// Register wires system metrics endpoints into Huma API.
func (h *SystemMetricsHandler) Register(api huma.API) {
	huma.Get(api, "/v1/stats/system", h.GetSystemMetrics)
}

// GetSystemMetrics returns current CPU, memory, network and connection stats.
func (h *SystemMetricsHandler) GetSystemMetrics(ctx context.Context, _ *struct{}) (*SystemMetricsOutput, error) {
	return &SystemMetricsOutput{Body: h.latest()}, nil
}

// StreamSystemMetricsHandler serves an SSE stream of system metrics snapshots.
type StreamSystemMetricsHandler struct {
	handler  *SystemMetricsHandler
	interval time.Duration
	logger   *slog.Logger
}

// NewStreamSystemMetricsHandler creates an SSE system metrics handler.
func NewStreamSystemMetricsHandler(handler *SystemMetricsHandler, logger *slog.Logger) *StreamSystemMetricsHandler {
	return &StreamSystemMetricsHandler{handler: handler, interval: 2 * time.Second, logger: logger}
}

// ServeHTTP implements http.Handler for SSE system metrics streaming.
func (h *StreamSystemMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		data := h.handler.latest()
		payload, err := json.Marshal(data)
		if err != nil {
			h.logger.Error("failed to marshal system metrics", slog.String("error", err.Error()))
			return
		}

		_, err = fmt.Fprintf(w, "data: %s\n\n", string(payload))
		if err != nil {
			return
		}
		flusher.Flush()

		select {
		case <-ticker.C:
		case <-r.Context().Done():
			return
		}
	}
}
