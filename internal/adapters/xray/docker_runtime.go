package xray

import (
	"fmt"
	"log/slog"
	"os/exec"
)

// DockerRuntimeController uses docker CLI to send HUP signal to Xray container.
// Requires docker.sock to be mounted in the hub container.
type DockerRuntimeController struct {
	logger      *slog.Logger
	containerID string
}

// NewDockerRuntimeController creates a new docker-based runtime controller.
// containerID is the Docker container ID or name of the Xray container.
func NewDockerRuntimeController(logger *slog.Logger, containerID string) *DockerRuntimeController {
	return &DockerRuntimeController{
		logger:      logger,
		containerID: containerID,
	}
}

// Start initializes the docker runtime controller.
func (c *DockerRuntimeController) Start(configPath string) error {
	c.logger.Info("docker runtime controller initialized", slog.String("container_id", c.containerID))
	return nil
}

// Reload restarts Xray container via docker restart.
func (c *DockerRuntimeController) Reload(configPath string) error {
	// Restart container to reload config (Xray doesn't support HUP for config reload)
	cmd := exec.Command("docker", "restart", c.containerID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Warn("failed to restart xray container",
			slog.String("container_id", c.containerID),
			slog.String("error", err.Error()),
			slog.String("output", string(output)),
		)
		return fmt.Errorf("restarting xray container: %w", err)
	}

	c.logger.Info("xray config reloaded via docker restart")
	return nil
}

// Stop is a no-op for docker controller.
func (c *DockerRuntimeController) Stop() {}

// Description returns a description of the controller.
func (c *DockerRuntimeController) Description() string {
	return "docker"
}
