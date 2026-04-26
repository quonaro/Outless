package xray

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"outless/pkg/config"
	"outless/pkg/logging"

	"gopkg.in/natefinch/lumberjack.v2"
)

// EmbeddedProbeRuntime manages a single Xray probe child process.
type EmbeddedProbeRuntime struct {
	logger      *slog.Logger
	binary      string
	adminPort   int
	socksPort   int
	configPath  string
	xrayLogPath string
	rotationCfg config.RotationConfig
	workerIndex int

	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewEmbeddedProbeRuntime creates a new embedded probe runtime.
func NewEmbeddedProbeRuntime(logger *slog.Logger, binary string, adminPort, socksPort int, configPath, xrayLogPath string, rotationCfg config.RotationConfig, workerIndex int) *EmbeddedProbeRuntime {
	if binary == "" {
		binary = "xray"
	}
	if adminPort == 0 {
		adminPort = 10085
	}
	if socksPort == 0 {
		socksPort = 1080
	}
	if configPath == "" {
		configPath = "/tmp/xray-probe-config.json"
	}
	return &EmbeddedProbeRuntime{
		logger:      logger,
		binary:      binary,
		adminPort:   adminPort,
		socksPort:   socksPort,
		configPath:  configPath,
		xrayLogPath: xrayLogPath,
		rotationCfg: rotationCfg,
		workerIndex: workerIndex,
	}
}

// Start generates config and starts the Xray probe process.
func (r *EmbeddedProbeRuntime) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil {
		return errors.New("probe already running")
	}

	config, err := r.generateConfig()
	if err != nil {
		return fmt.Errorf("generating probe config: %w", err)
	}

	if err := os.WriteFile(r.configPath, config, 0o600); err != nil {
		return fmt.Errorf("writing probe config: %w", err)
	}

	cmd := exec.Command(r.binary, "run", "-c", r.configPath)

	// Setup logging to file with rotation if path is specified
	if r.xrayLogPath != "" {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(r.xrayLogPath), 0755); err != nil {
			r.logger.Warn("failed to create xray log directory", slog.String("error", err.Error()))
		} else {
			// Use lumberjack for rotation
			lumberjackLogger := &lumberjack.Logger{
				Filename:   r.xrayLogPath,
				MaxSize:    r.rotationCfg.MaxSizeMB,
				MaxBackups: r.rotationCfg.MaxBackups,
				MaxAge:     r.rotationCfg.MaxAgeDays,
				Compress:   r.rotationCfg.Compress,
			}
			// Create prefixed writer for probe logs with worker index
			prefix := fmt.Sprintf("xray_probe_%d: ", r.workerIndex)
			prefixedWriter := logging.NewPrefixWriter(prefix, lumberjackLogger)
			cmd.Stdout = prefixedWriter
			cmd.Stderr = prefixedWriter
		}
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting probe process: %w", err)
	}

	r.cmd = cmd
	r.logger.Info("embedded probe started",
		slog.Int("pid", cmd.Process.Pid),
		slog.Int("admin_port", r.adminPort),
		slog.Int("socks_port", r.socksPort),
		slog.String("config_path", r.configPath),
	)

	// Wait for Xray to be ready
	if err := r.waitForReady(ctx); err != nil {
		_ = r.Stop()
		return fmt.Errorf("probe not ready: %w", err)
	}

	return nil
}

// Stop terminates the probe process.
func (r *EmbeddedProbeRuntime) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		r.logger.Warn("probe interrupt failed", slog.String("error", err.Error()))
		_ = r.cmd.Process.Kill()
	}
	_ = r.cmd.Wait()
	r.cmd = nil

	r.logger.Info("embedded probe stopped")
	return nil
}

// AdminURL returns the gRPC API URL for this probe.
func (r *EmbeddedProbeRuntime) AdminURL() string {
	return fmt.Sprintf("127.0.0.1:%d", r.adminPort)
}

// SocksAddr returns the SOCKS address for this probe.
func (r *EmbeddedProbeRuntime) SocksAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", r.socksPort)
}

// generateConfig creates minimal Xray config for probe operations.
func (r *EmbeddedProbeRuntime) generateConfig() ([]byte, error) {
	cfg := map[string]any{
		"log": map[string]any{
			"loglevel": "warning",
		},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"HandlerService", "LoggerService", "StatsService", "RoutingService"},
		},
		"inbounds": []any{
			map[string]any{
				"tag":      "api-in",
				"listen":   "127.0.0.1",
				"port":     r.adminPort,
				"protocol": "dokodemo-door",
				"settings": map[string]any{
					"address": "127.0.0.1",
				},
			},
			map[string]any{
				"tag":      "socks-in",
				"listen":   "127.0.0.1",
				"port":     r.socksPort,
				"protocol": "socks",
				"settings": map[string]any{
					"udp": true,
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls"},
				},
			},
		},
		"outbounds": []any{
			map[string]any{
				"tag":      "direct",
				"protocol": "freedom",
			},
		},
		"routing": map[string]any{
			"rules": []any{
				map[string]any{
					"type":        "field",
					"inboundTag":  []string{"api-in"},
					"outboundTag": "api",
				},
			},
		},
	}

	return json.MarshalIndent(cfg, "", "  ")
}

// waitForReady waits for the Xray gRPC API to become available.
func (r *EmbeddedProbeRuntime) waitForReady(ctx context.Context) error {
	addr := fmt.Sprintf("127.0.0.1:%d", r.adminPort)

	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
	}

	return fmt.Errorf("probe API not ready at %s", addr)
}
