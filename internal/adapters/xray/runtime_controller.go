package xray

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
)

// EmbeddedRuntimeController starts and owns a local Xray child process.
type EmbeddedRuntimeController struct {
	logger *slog.Logger
	binary string

	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewEmbeddedRuntimeController creates runtime controller for embedded mode.
func NewEmbeddedRuntimeController(logger *slog.Logger, binary string) *EmbeddedRuntimeController {
	if binary == "" {
		binary = "xray"
	}
	return &EmbeddedRuntimeController{
		logger: logger,
		binary: binary,
	}
}

func (c *EmbeddedRuntimeController) Start(configPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd != nil {
		return errors.New("xray already running")
	}
	cmd := exec.Command(c.binary, "run", "-c", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting xray process: %w", err)
	}

	c.cmd = cmd
	c.logger.Info("embedded xray started",
		slog.Int("pid", cmd.Process.Pid),
		slog.String("binary", c.binary),
		slog.String("config_path", configPath),
	)
	return nil
}

func (c *EmbeddedRuntimeController) Reload(configPath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return nil
	}

	if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
	c.cmd = nil

	cmd := exec.Command(c.binary, "run", "-c", configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restarting xray: %w", err)
	}
	c.cmd = cmd
	c.logger.Info("embedded xray restarted",
		slog.Int("pid", cmd.Process.Pid),
		slog.String("config_path", configPath),
	)
	return nil
}

func (c *EmbeddedRuntimeController) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cmd == nil || c.cmd.Process == nil {
		return
	}
	if err := c.cmd.Process.Signal(os.Interrupt); err != nil {
		c.logger.Warn("embedded xray interrupt failed", slog.String("error", err.Error()))
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
	c.cmd = nil
}

func (c *EmbeddedRuntimeController) Description() string {
	return "embedded"
}

// ExternalRuntimeController assumes edge Xray lifecycle is managed outside hub process.
type ExternalRuntimeController struct {
	logger *slog.Logger
}

// NewExternalRuntimeController creates runtime controller for external mode.
func NewExternalRuntimeController(logger *slog.Logger) *ExternalRuntimeController {
	return &ExternalRuntimeController{logger: logger}
}

func (c *ExternalRuntimeController) Start(configPath string) error {
	c.logger.Info("external xray runtime selected: start is delegated",
		slog.String("config_path", configPath),
	)
	return nil
}

func (c *ExternalRuntimeController) Reload(configPath string) error {
	// Send SIGHUP to xray-edge container to trigger config reload
	// The container name is hardcoded as per docker-compose.yaml
	cmd := exec.Command("docker", "kill", "--signal=HUP", "outless-xray-edge")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Warn("failed to send HUP to xray-edge container",
			slog.String("error", err.Error()),
			slog.String("output", string(output)),
		)
		return fmt.Errorf("sending HUP to xray-edge: %w", err)
	}

	c.logger.Info("external xray runtime: HUP signal sent to container",
		slog.String("config_path", configPath),
	)
	return nil
}

func (c *ExternalRuntimeController) Stop() {}

func (c *ExternalRuntimeController) Description() string {
	return "external"
}
