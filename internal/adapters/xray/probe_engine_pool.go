package xray

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"outless/internal/domain"
	"outless/pkg/config"
)

// ProbeEnginePool dispatches ProbeNode calls across independent probe shards.
type ProbeEnginePool struct {
	engines []domain.ProxyEngine
	next    atomic.Uint64
}

// NewProbeEnginePool builds one Engine per configured shard.
func NewProbeEnginePool(
	logger *slog.Logger,
	probeURL string,
	shards []config.XrayProbeShardConfig,
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

// ProbeNode executes probe on next shard in round-robin order.
func (p *ProbeEnginePool) ProbeNode(ctx context.Context, node domain.Node) (domain.ProbeResult, error) {
	if p == nil || len(p.engines) == 0 {
		return domain.ProbeResult{}, errors.New("probe engine pool is not configured")
	}
	idx := int(p.next.Add(1)-1) % len(p.engines)
	result, err := p.engines[idx].ProbeNode(ctx, node)
	if err != nil {
		return domain.ProbeResult{}, fmt.Errorf("probe shard %d: %w", idx, err)
	}
	return result, nil
}
