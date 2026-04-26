package xray

import (
	"context"
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

// EmbeddedHubRuntime manages the Xray hub child process for routing traffic.
type EmbeddedHubRuntime struct {
	logger      *slog.Logger
	binary      string
	configPath  string
	xrayLogPath string
	rotationCfg config.RotationConfig

	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewEmbeddedHubRuntime creates a new embedded hub runtime.
func NewEmbeddedHubRuntime(logger *slog.Logger, binary, configPath, xrayLogPath string, rotationCfg config.RotationConfig) *EmbeddedHubRuntime {
	if binary == "" {
		binary = "xray"
	}
	if configPath == "" {
		configPath = "/var/lib/outless/xray-hub.json"
	}
	return &EmbeddedHubRuntime{
		logger:      logger,
		binary:      binary,
		configPath:  configPath,
		xrayLogPath: xrayLogPath,
		rotationCfg: rotationCfg,
	}
}

// Start starts the Xray hub process with the given config path.
// Must NOT be called while r.mu is held.
func (r *EmbeddedHubRuntime) Start(configPath string) error {
	r.mu.Lock()
	if r.cmd != nil {
		r.mu.Unlock()
		r.logger.Warn("hub runtime already running")
		return nil
	}

	r.configPath = configPath

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
			// Create prefixed writer for hub logs
			prefixedWriter := logging.NewPrefixWriter("xray_hub_1: ", lumberjackLogger)
			cmd.Stdout = prefixedWriter
			cmd.Stderr = prefixedWriter
		}
	}

	if err := cmd.Start(); err != nil {
		r.mu.Unlock()
		return err
	}

	r.cmd = cmd
	pid := cmd.Process.Pid
	r.mu.Unlock() // release BEFORE waitForReady so it can read r.cmd safely

	r.logger.Info("xray hub runtime started",
		slog.Int("pid", pid),
		slog.String("config_path", configPath),
	)

	// Wait for Xray to bind its port — must happen WITHOUT holding r.mu.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r.waitForReady(ctx)

	return nil
}

// Reload restarts the Xray hub process with the updated config.
// Xray does not support live config reload — killing the process ensures all
// authenticated TCP connections are dropped and the new client list takes effect.
//
// Must NOT be called while r.mu is held.
func (r *EmbeddedHubRuntime) Reload(configPath string) error {
	r.mu.Lock()

	if r.cmd == nil || r.cmd.Process == nil {
		r.mu.Unlock()
		r.logger.Warn("hub runtime not running, starting fresh")
		return r.Start(configPath)
	}

	r.logger.Info("xray hub runtime: restarting for config reload",
		slog.String("config_path", configPath))
	r.stopLocked() // kills process, sets r.cmd = nil
	r.mu.Unlock()  // release BEFORE Start (which acquires the lock)

	// Brief pause to let the OS reclaim the listening port.
	time.Sleep(300 * time.Millisecond)

	return r.Start(configPath)
}

// Stop terminates the Xray hub process.
func (r *EmbeddedHubRuntime) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

// stopLocked kills the Xray process and waits for it to exit.
// Caller MUST hold r.mu.
func (r *EmbeddedHubRuntime) stopLocked() {
	if r.cmd == nil || r.cmd.Process == nil {
		return
	}
	if err := r.cmd.Process.Kill(); err != nil {
		r.logger.Warn("hub kill failed", slog.String("error", err.Error()))
	}
	_ = r.cmd.Wait()
	r.cmd = nil
	r.logger.Info("xray hub runtime stopped")
}

// Description returns a description of the runtime controller.
func (r *EmbeddedHubRuntime) Description() string {
	return "embedded-xray-hub"
}

// waitForReady polls until Xray's inbound port accepts connections or the
// context expires. Must NOT be called while r.mu is held.
func (r *EmbeddedHubRuntime) waitForReady(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}

		conn, err := net.DialTimeout("tcp", "127.0.0.1:443", 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
	}
}
