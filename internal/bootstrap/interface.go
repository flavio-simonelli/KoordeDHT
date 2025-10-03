package bootstrap

import (
	"KoordeDHT/internal/domain"
	"context"
)

type Bootstrap interface {
	// Discover returns a list of known peer addresses
	Discover(ctx context.Context) ([]string, error)
	// Register add the current node (only if needed, e.g. Route53)
	Register(ctx context.Context, node *domain.Node) error
	// Deregister remove the current node (only if needed, e.g. Route53)
	Deregister(ctx context.Context, node *domain.Node) error
}
