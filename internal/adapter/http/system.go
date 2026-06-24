package http

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"

	"outless/internal/domain"
)

// SystemMetricsHandler serves live system metrics.
type SystemMetricsHandler struct {
	runtime     domain.RuntimeController
	logger      *slog.Logger
	mu          sync.Mutex
	lastNetRX   uint64
	lastNetTX   uint64
	lastNetTime time.Time
}

// NewSystemMetricsHandler creates a system metrics handler.
func NewSystemMetricsHandler(runtime domain.RuntimeController, logger *slog.Logger) *SystemMetricsHandler {
	return &SystemMetricsHandler{runtime: runtime, logger: logger}
}

// SystemMetricsOutput is the JSON payload returned by GET /v1/stats/system.
type SystemMetricsOutput struct {
	Body struct {
		CPUPercent       float64 `json:"cpu_percent"`
		MemoryPercent    float64 `json:"memory_percent"`
		MemoryUsedBytes  uint64  `json:"memory_used_bytes"`
		MemoryTotalBytes uint64  `json:"memory_total_bytes"`
		NetRXBytesPerSec float64 `json:"net_rx_bytes_per_sec"`
		NetTXBytesPerSec float64 `json:"net_tx_bytes_per_sec"`
		ConnectionsCount int     `json:"connections_count"`
	}
}

// Register wires system metrics endpoints into Huma API.
func (h *SystemMetricsHandler) Register(api huma.API) {
	huma.Get(api, "/v1/stats/system", h.GetSystemMetrics)
}

// GetSystemMetrics returns current CPU, memory, network and connection stats.
func (h *SystemMetricsHandler) GetSystemMetrics(ctx context.Context, _ *struct{}) (*SystemMetricsOutput, error) {
	out := &SystemMetricsOutput{}

	cpuPercents, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err != nil {
		h.logger.Warn("failed to get cpu percent", slog.String("error", err.Error()))
	} else if len(cpuPercents) > 0 {
		out.Body.CPUPercent = cpuPercents[0]
	}

	vmStat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		h.logger.Warn("failed to get memory stats", slog.String("error", err.Error()))
	} else if vmStat != nil {
		out.Body.MemoryPercent = vmStat.UsedPercent
		out.Body.MemoryUsedBytes = vmStat.Used
		out.Body.MemoryTotalBytes = vmStat.Total
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
			out.Body.NetRXBytesPerSec = 0
			out.Body.NetTXBytesPerSec = 0
		} else {
			out.Body.NetRXBytesPerSec = float64(totalRX-h.lastNetRX) / elapsed
			out.Body.NetTXBytesPerSec = float64(totalTX-h.lastNetTX) / elapsed
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
		out.Body.ConnectionsCount = len(uniqueUsers)
	}

	return out, nil
}
