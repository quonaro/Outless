package xray

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"outless/internal/domain"
)

// ProbeShardConfig defines connection details for a single probe runtime.
type ProbeShardConfig struct {
	AdminURL  string
	SocksAddr string
}

// ProbeEnginePool dispatches ProbeNode calls across independent probe shards.
type ProbeEnginePool struct {
	engines []domain.ProxyEngine
	next    atomic.Uint64
}

// NewProbeEnginePool builds one Engine per configured shard.
func NewProbeEnginePool(
	logger *slog.Logger,
	probeURL string,
	shards []ProbeShardConfig,
	geoIP GeoIPConfig,
	probeTimeout time.Duration,
) (*ProbeEnginePool, error) {
	if len(shards) == 0 {
		return nil, errors.New("at least one xray probe shard is required")
	}

	engines := make([]domain.ProxyEngine, 0, len(shards))
	for i, shard := range shards {
		engine := NewEngine(logger, probeURL, shard.AdminURL, shard.SocksAddr, geoIP, probeTimeout)
		engines = append(engines, engine)
		if logger != nil {
			logger.Info("xray probe shard configured",
				slog.Int("shard_index", i),
				slog.String("admin_url", shard.AdminURL),
				slog.String("socks_addr", shard.SocksAddr),
			)
		}
	}

	return &ProbeEnginePool{engines: engines}, nil
}

// ProbeNode executes probe on next shard in round-robin order with retry on failure.
func (p *ProbeEnginePool) ProbeNode(ctx context.Context, node domain.Node) (domain.ProbeResult, error) {
	if p == nil || len(p.engines) == 0 {
		return domain.ProbeResult{}, errors.New("probe engine pool is not configured")
	}

	// Retry up to 3 times with different shards
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		idx := int(p.next.Add(1)-1) % len(p.engines)
		result, err := p.engines[idx].ProbeNode(ctx, node)
		if err == nil {
			return result, nil
		}
		lastErr = fmt.Errorf("probe shard %d (attempt %d): %w", idx, attempt+1, err)
	}

	return domain.ProbeResult{}, lastErr
}
