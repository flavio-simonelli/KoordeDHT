package bootstrap

import (
	"KoordeDHT/internal/config"
	"fmt"
	"net"
	"strings"
)

// ResolveBootstrap resolves bootstrap peers into a list of "host:port" addresses.
//
// Rules:
//   - mode=static → returns the configured peers.
//   - mode=dns    → resolves peers via DNS (SRV or A/AAAA).
//   - if resolution returns an empty list, it's not an error: it means
//     the current node may start a new DHT as the first node.
func ResolveBootstrap(cfg config.BootstrapConfig) ([]string, error) {
	switch cfg.Mode {
	case "static":
		return cfg.Peers, nil
	case "dns":
		if cfg.SRV {
			_, addrs, err := net.LookupSRV("", "", cfg.DNSName)
			if err != nil {
				return nil, fmt.Errorf("SRV lookup failed: %w", err)
			}
			out := make([]string, 0, len(addrs))
			for _, srv := range addrs {
				target := strings.TrimSuffix(srv.Target, ".")
				out = append(out, fmt.Sprintf("%s:%d", target, srv.Port))
			}
			return out, nil
		}
		hosts, err := net.LookupHost(cfg.DNSName)
		if err != nil {
			return nil, fmt.Errorf("A/AAAA lookup failed: %w", err)
		}
		out := make([]string, 0, len(hosts))
		for _, h := range hosts {
			out = append(out, fmt.Sprintf("%s:%d", h, cfg.Port))
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported bootstrap mode: %s", cfg.Mode)
	}
}
