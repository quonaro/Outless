package xray

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"outless/pkg/config"
)

// ProbeRuntimePool manages multiple embedded Xray probe processes.
type ProbeRuntimePool struct {
	logger      *slog.Logger
	binary      string
	basePort    int
	configDir   string
	xrayLogPath string
	rotationCfg config.RotationConfig

	mu       sync.Mutex
	runtimes []*EmbeddedProbeRuntime
	next     atomic.Uint64
}

// NewProbeRuntimePool creates a pool for managing N probe processes.
// basePort is the starting port number (admin port), subsequent probes use basePort + 2*i
func NewProbeRuntimePool(logger *slog.Logger, binary string, basePort int, configDir, xrayLogPath string, rotationCfg config.RotationConfig) *ProbeRuntimePool {
	if binary == "" {
		binary = "xray"
	}
	if basePort == 0 {
		basePort = 10085
	}
	if configDir == "" {
		configDir = "/tmp/outless-probe"
	}
	return &ProbeRuntimePool{
		logger:      logger,
		binary:      binary,
		basePort:    basePort,
		configDir:   configDir,
		xrayLogPath: xrayLogPath,
		rotationCfg: rotationCfg,
	}
}

// Start initializes N probe processes.
func (p *ProbeRuntimePool) Start(ctx context.Context, count int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.runtimes) > 0 {
		return fmt.Errorf("pool already started with %d runtimes", len(p.runtimes))
	}

	if err := os.MkdirAll(p.configDir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	p.runtimes = make([]*EmbeddedProbeRuntime, count)

	for i := 0; i < count; i++ {
		adminPort := p.basePort + i*2
		socksPort := p.basePort + i*2 + 1
		configPath := filepath.Join(p.configDir, fmt.Sprintf("probe-%d.json", i))

		runtime := NewEmbeddedProbeRuntime(p.logger, p.binary, adminPort, socksPort, configPath, p.xrayLogPath, p.rotationCfg, i)
		if err := runtime.Start(ctx); err != nil {
			// Cleanup already started runtimes
			for j := 0; j < i; j++ {
				_ = p.runtimes[j].Stop()
			}
			p.runtimes = nil
			return fmt.Errorf("starting probe %d: %w", i, err)
		}

		p.runtimes[i] = runtime
		p.logger.Info("probe runtime started",
			slog.Int("index", i),
			slog.Int("admin_port", adminPort),
			slog.Int("socks_port", socksPort),
		)
	}

	return nil
}

// Stop terminates all probe processes.
func (p *ProbeRuntimePool) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for _, runtime := range p.runtimes {
		if err := runtime.Stop(); err != nil {
			lastErr = err
		}
	}
	p.runtimes = nil

	return lastErr
}

// GetRuntime returns the next probe runtime in round-robin order.
func (p *ProbeRuntimePool) GetRuntime() *EmbeddedProbeRuntime {
	if len(p.runtimes) == 0 {
		return nil
	}
	idx := int(p.next.Add(1)-1) % len(p.runtimes)
	return p.runtimes[idx]
}

// ShardConfigs returns shard configs compatible with ProbeEnginePool.
func (p *ProbeRuntimePool) ShardConfigs() []ProbeShardConfig {
	p.mu.Lock()
	defer p.mu.Unlock()

	configs := make([]ProbeShardConfig, len(p.runtimes))
	for i, runtime := range p.runtimes {
		configs[i] = ProbeShardConfig{
			AdminURL:  "http://" + runtime.AdminURL(),
			SocksAddr: runtime.SocksAddr(),
		}
	}
	return configs
}

// Count returns the number of active probe runtimes.
func (p *ProbeRuntimePool) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.runtimes)
}
