package tester

import (
	"context"
	"fmt"
	"strings"

	"KoordeDHT/internal/domain"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerBootstrap discovers nodes by container name prefix and network.
type DockerBootstrap struct {
	Prefix  string // e.g. "localtest-node"
	Port    int    // e.g. 4000
	Network string // e.g. "koorde-net"
}

// NewDockerBootstrap creates a Docker-based bootstrapper.
func NewDockerBootstrap(suffix string, port int, network string) *DockerBootstrap {
	return &DockerBootstrap{
		Prefix:  strings.TrimSpace(suffix),
		Port:    port,
		Network: strings.TrimSpace(network),
	}
}

// Discover returns a list of reachable peers by container name prefix.
// Since the tester runs in the same Docker network, it returns hostnames only.
func (d *DockerBootstrap) Discover(ctx context.Context) ([]string, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client init failed: %w", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var peers []string
	for _, c := range containers {
		if len(c.Names) == 0 {
			continue
		}

		name := strings.TrimPrefix(c.Names[0], "/")
		if strings.HasPrefix(name, d.Prefix) {
			peers = append(peers, fmt.Sprintf("%s:%d", name, d.Port))
		}
	}

	if len(peers) == 0 {
		return nil, fmt.Errorf("no active containers found with prefix %q", d.Prefix)
	}

	return peers, nil
}

// Register and Deregister are no-ops
func (d *DockerBootstrap) Register(ctx context.Context, node *domain.Node) error   { return nil }
func (d *DockerBootstrap) Deregister(ctx context.Context, node *domain.Node) error { return nil }
