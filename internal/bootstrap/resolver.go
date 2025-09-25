package bootstrap

import (
	"KoordeDHT/internal/config"
	"fmt"
	"math/rand"
	"net"
	"strings"
)

// ResolveBootstrap restituisce una lista di indirizzi "host:porta" dai bootstrap config.
func ResolveBootstrap(cfg config.BootstrapConfig) ([]string, error) {
	switch cfg.Mode {
	case "init":
		// Primo nodo: nessun peer, ma non è un errore
		return nil, nil
	case "static":
		return cfg.Peers, nil
	case "dns":
		if cfg.SRV {
			// Lookup SRV record: es. _koorde._tcp.bootstrap.koorde.local
			_, addrs, err := net.LookupSRV("koorde", "tcp", cfg.DNSName)
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
		// Lookup A/AAAA record: es. bootstrap.koorde.local → IP
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

// PickRandom sceglie casualmente un indirizzo dalla lista.
// Restituisce errore se la lista è vuota.
func PickRandom(peers []string) (string, error) {
	if len(peers) == 0 {
		return "", fmt.Errorf("no bootstrap peers available")
	}
	idx := rand.Intn(len(peers))
	return peers[idx], nil
}
