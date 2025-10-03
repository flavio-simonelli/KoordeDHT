package bootstrap

import (
	"KoordeDHT/internal/config"
	"KoordeDHT/internal/logger"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// ResolveBootstrap resolves bootstrap peers into a list of "host:port" addresses.
//
// Behavior:
//   - mode=static → returns the configured peers.
//   - mode=dns    → resolves peers via DNS (SRV or A/AAAA).
//   - If DNS resolution fails or returns no records, returns an empty list (not an error).
func ResolveBootstrap(cfg config.BootstrapConfig, lgr logger.Logger) ([]string, error) {
	switch cfg.Mode {
	case "static":
		return cfg.Peers, nil

	case "dns":
		client := &dns.Client{Timeout: 2 * time.Second}

		server := cfg.Resolver
		if server == "" {
			server = "8.8.8.8:53" // default fallback
		} else if !strings.Contains(server, ":") {
			server += ":53"
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// --- SRV lookup ---
		if cfg.SRV {
			name := fmt.Sprintf("_%s._%s.%s", cfg.Service, cfg.Proto, cfg.DNSName)
			msg := new(dns.Msg)
			msg.SetQuestion(dns.Fqdn(name), dns.TypeSRV)
			lgr.Info("Sending SRV query", logger.F("qname", msg.Question[0].Name))

			in, _, err := client.ExchangeContext(ctx, msg, server)
			if err != nil {
				lgr.Warn("SRV lookup failed", logger.F("err", err), logger.F("qname", name))
				return []string{}, nil
			}
			if len(in.Answer) == 0 {
				lgr.Warn("SRV lookup returned no answers", logger.F("qname", name))
				return []string{}, nil
			}

			// mappa nome → IPs dalla sezione "Additional"
			srvTargets := map[string][]string{}
			for _, extra := range in.Extra {
				switch rr := extra.(type) {
				case *dns.A:
					name := strings.TrimSuffix(rr.Hdr.Name, ".")
					srvTargets[name] = append(srvTargets[name], rr.A.String())
				case *dns.AAAA:
					name := strings.TrimSuffix(rr.Hdr.Name, ".")
					srvTargets[name] = append(srvTargets[name], rr.AAAA.String())
				}
			}

			out := []string{}
			for _, ans := range in.Answer {
				srv, ok := ans.(*dns.SRV)
				if !ok {
					continue
				}
				target := strings.TrimSuffix(srv.Target, ".")
				ips, found := srvTargets[target]

				if !found {
					// fallback: query A/AAAA
					msgA := new(dns.Msg)
					msgA.SetQuestion(dns.Fqdn(target), dns.TypeA)
					if inA, _, errA := client.ExchangeContext(ctx, msgA, server); errA == nil {
						for _, a := range inA.Answer {
							if arec, ok := a.(*dns.A); ok {
								ips = append(ips, arec.A.String())
							}
						}
					}
					msgAAAA := new(dns.Msg)
					msgAAAA.SetQuestion(dns.Fqdn(target), dns.TypeAAAA)
					if inAAAA, _, errAAAA := client.ExchangeContext(ctx, msgAAAA, server); errAAAA == nil {
						for _, a := range inAAAA.Answer {
							if aaaa, ok := a.(*dns.AAAA); ok {
								ips = append(ips, aaaa.AAAA.String())
							}
						}
					}
				}

				for _, ip := range ips {
					if strings.Contains(ip, ":") { // IPv6
						out = append(out, fmt.Sprintf("[%s]:%d", ip, srv.Port))
					} else {
						out = append(out, fmt.Sprintf("%s:%d", ip, srv.Port))
					}
				}
			}
			return out, nil
		}

		// --- A/AAAA lookup ---
		name := dns.Fqdn(cfg.DNSName)
		msg := new(dns.Msg)
		msg.SetQuestion(name, dns.TypeA)

		in, _, err := client.ExchangeContext(ctx, msg, server)
		if err != nil {
			lgr.Warn("A lookup failed", logger.F("err", err), logger.F("qname", name))
			return []string{}, nil
		}

		out := []string{}
		for _, ans := range in.Answer {
			if a, ok := ans.(*dns.A); ok {
				out = append(out, fmt.Sprintf("%s:%d", a.A.String(), cfg.Port))
			}
		}

		// fallback AAAA
		if len(out) == 0 {
			msg6 := new(dns.Msg)
			msg6.SetQuestion(name, dns.TypeAAAA)
			if in6, _, err6 := client.ExchangeContext(ctx, msg6, server); err6 == nil {
				for _, ans := range in6.Answer {
					if aaaa, ok := ans.(*dns.AAAA); ok {
						out = append(out, fmt.Sprintf("[%s]:%d", aaaa.AAAA.String(), cfg.Port))
					}
				}
			}
		}

		if len(out) == 0 {
			lgr.Warn("Host lookup returned no addresses", logger.F("qname", name))
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported bootstrap mode: %s", cfg.Mode)
	}
}
