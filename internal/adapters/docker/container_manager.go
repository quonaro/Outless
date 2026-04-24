package docker

import (
	"context"
	"fmt"
	"log/slog"

	"outless/internal/domain"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	xrayImage       = "ghcr.io/xtls/xray-core:latest"
	adminPort       = 10085
	socksPort       = 1080
	probeConfigPath = "/etc/xray/config.json"
	hostConfigPath  = "./xray/config.json"
)

// ContainerManager manages Docker containers for Xray probe shards.
type ContainerManager struct {
	cli    *client.Client
	logger *slog.Logger
}

// NewContainerManager creates a new Docker container manager.
func NewContainerManager(logger *slog.Logger) (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &ContainerManager{
		cli:    cli,
		logger: logger,
	}, nil
}

// CreateProbeContainer creates a new Xray probe container with the given name.
func (m *ContainerManager) CreateProbeContainer(ctx context.Context, name string) error {
	m.logger.Info("creating probe container", slog.String("name", name))

	config := &container.Config{
		Image: xrayImage,
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", adminPort)): struct{}{},
			nat.Port(fmt.Sprintf("%d/tcp", socksPort)): struct{}{},
		},
		Cmd: []string{"run", "-config", probeConfigPath},
	}

	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   hostConfigPath,
				Target:   probeConfigPath,
				ReadOnly: true,
			},
		},
	}

	resp, err := m.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, name)
	if err != nil {
		return fmt.Errorf("creating container %s: %w", name, err)
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}

	m.logger.Info("probe container created and started", slog.String("name", name), slog.String("id", resp.ID))
	return nil
}

// RemoveProbeContainer removes the probe container with the given name.
func (m *ContainerManager) RemoveProbeContainer(ctx context.Context, name string) error {
	m.logger.Info("removing probe container", slog.String("name", name))

	if err := m.cli.ContainerRemove(ctx, name, container.RemoveOptions{
		Force: true,
	}); err != nil {
		return fmt.Errorf("removing container %s: %w", name, err)
	}

	m.logger.Info("probe container removed", slog.String("name", name))
	return nil
}

// ListProbeContainers lists all probe containers (xray-probe-*).
func (m *ContainerManager) ListProbeContainers(ctx context.Context) ([]string, error) {
	filter := filters.NewArgs()
	filter.Add("name", "xray-probe-")

	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		Filters: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	names := make([]string, 0, len(containers))
	for _, c := range containers {
		for _, name := range c.Names {
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
			names = append(names, name)
		}
	}

	return names, nil
}

// ContainerExists checks if a container with the given name exists.
func (m *ContainerManager) ContainerExists(ctx context.Context, name string) (bool, error) {
	_, err := m.cli.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspecting container %s: %w", name, err)
	}
	return true, nil
}

// Close closes the Docker client connection.
func (m *ContainerManager) Close() error {
	if m.cli != nil {
		return m.cli.Close()
	}
	return nil
}

var _ domain.DockerContainerManager = (*ContainerManager)(nil)
