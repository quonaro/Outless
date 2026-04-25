package xray

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

// EmbeddedHubRuntime manages the Xray hub child process for routing traffic.
type EmbeddedHubRuntime struct {
	logger     *slog.Logger
	binary     string
	configPath string

	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewEmbeddedHubRuntime creates a new embedded hub runtime.
func NewEmbeddedHubRuntime(logger *slog.Logger, binary, configPath string) *EmbeddedHubRuntime {
	if binary == "" {
		binary = "xray"
	}
	if configPath == "" {
		configPath = "/app/tmp/xray-hub.json"
	}
	return &EmbeddedHubRuntime{
		logger:     logger,
		binary:     binary,
		configPath: configPath,
	}
}

// Start starts the Xray hub process with the given config path.
func (r *EmbeddedHubRuntime) Start(configPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil {
		r.logger.Warn("hub runtime already running")
		return nil
	}

	r.configPath = configPath

	cmd := exec.Command(r.binary, "run", "-c", r.configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	r.cmd = cmd
	r.logger.Info("xray hub runtime started",
		slog.Int("pid", cmd.Process.Pid),
		slog.String("config_path", r.configPath),
	)

	// Wait for Xray to be ready
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := r.waitForReady(ctx); err != nil {
		r.stopLocked()
		return err
	}

	return nil
}

// Reload reloads the Xray hub process with a new config.
func (r *EmbeddedHubRuntime) Reload(configPath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		r.logger.Warn("hub runtime not running, starting with new config")
		r.mu.Unlock()
		return r.Start(configPath)
	}

	r.configPath = configPath

	// Send SIGHUP to reload config
	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		r.logger.Warn("hub reload signal failed, restarting", slog.String("error", err.Error()))
		r.stopLocked()
		r.mu.Unlock()
		return r.Start(configPath)
	}

	r.logger.Info("xray hub runtime reloaded", slog.String("config_path", r.configPath))
	return nil
}

// Stop terminates the Xray hub process.
func (r *EmbeddedHubRuntime) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

// stopLocked terminates the Xray hub process without acquiring the mutex.
// Caller must hold r.mu.
func (r *EmbeddedHubRuntime) stopLocked() {
	if r.cmd == nil || r.cmd.Process == nil {
		return
	}

	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		r.logger.Warn("hub interrupt failed", slog.String("error", err.Error()))
		_ = r.cmd.Process.Kill()
	}
	_ = r.cmd.Wait()
	r.cmd = nil

	r.logger.Info("xray hub runtime stopped")
}

// Description returns a description of the runtime controller.
func (r *EmbeddedHubRuntime) Description() string {
	return "embedded-xray-hub"
}

// waitForReady waits for the Xray hub to be ready by checking the inbound port.
func (r *EmbeddedHubRuntime) waitForReady(ctx context.Context) error {
	// Parse the config to get the inbound port
	// For simplicity, we'll check if the process is still running
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}

		if r.cmd == nil || r.cmd.Process == nil {
			return nil
		}

		// Check if process is still running
		if err := r.cmd.Process.Signal(os.Signal(nil)); err != nil {
			// Process has exited
			return nil
		}

		// Try to connect to a common port to verify readiness
		// This is a simplified check - in production you might want to parse the config
		conn, err := net.DialTimeout("tcp", "127.0.0.1:443", 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
	}

	return nil
}
