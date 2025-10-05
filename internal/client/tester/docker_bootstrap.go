package tester

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"KoordeDHT/internal/domain"
)

// DockerBootstrap discovers nodes by container name suffix and network.
type DockerBootstrap struct {
	Suffix  string // e.g. "localtest-node"
	Port    int    // e.g. 4000
	Network string // e.g. "koorde-net"
}

// NewDockerBootstrap creates a Docker-based bootstrapper.
func NewDockerBootstrap(suffix string, port int, network string) *DockerBootstrap {
	return &DockerBootstrap{
		Suffix:  strings.TrimSpace(suffix),
		Port:    port,
		Network: strings.TrimSpace(network),
	}
}

// Discover returns a list of reachable peers in the given Docker network.
func (d *DockerBootstrap) Discover(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.Names}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var addrs []string

	for _, name := range lines {
		name = strings.TrimSpace(name)
		if name == "" || !strings.Contains(name, d.Suffix) {
			continue
		}

		// Inspect container JSON for its network list
		inspect := exec.CommandContext(ctx, "docker", "inspect", name)
		raw, err := inspect.Output()
		if err != nil {
			continue
		}

		var data []struct {
			NetworkSettings struct {
				Networks map[string]struct {
					IPAddress string `json:"IPAddress"`
				} `json:"Networks"`
			} `json:"NetworkSettings"`
		}

		if err := json.Unmarshal(raw, &data); err != nil || len(data) == 0 {
			continue
		}

		// Filter by network name
		netInfo, ok := data[0].NetworkSettings.Networks[d.Network]
		if !ok || netInfo.IPAddress == "" {
			continue
		}

		addr := fmt.Sprintf("%s:%d", name, d.Port) // use name (DNS) instead of IP
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// Register and Deregister are no-ops
func (d *DockerBootstrap) Register(ctx context.Context, node *domain.Node) error   { return nil }
func (d *DockerBootstrap) Deregister(ctx context.Context, node *domain.Node) error { return nil }
