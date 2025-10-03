package register

import (
	"KoordeDHT/internal/config"
	"context"
	"fmt"
)

func NewRegistrar(ctx context.Context, cfg config.RegisterConfig) (Registrar, error) {
	switch cfg.Type {
	case "route53":
		return NewRoute53Registrar(ctx, cfg.Route53.HostedZoneID, cfg.Route53.DomainSuffix, cfg.TTL)

	case "coredns":
		basePath := cfg.CoreDNS.BasePath
		return NewCoreDNSRegistrar(cfg.CoreDNS.EtcdEndpoints, basePath, cfg.CoreDNS.Domain, cfg.TTL)

	default:
		return nil, fmt.Errorf("unsupported registrar type: %s", cfg.Type)
	}
}
