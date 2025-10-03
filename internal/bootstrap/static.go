package bootstrap

import (
	"KoordeDHT/internal/domain"
	"context"
)

// StaticBootstrap implements a static list of bootstrap peers
type StaticBootstrap struct {
	peers []string
}

func NewStaticBootstrap(peers []string) *StaticBootstrap {
	return &StaticBootstrap{peers: peers}
}

// Discover returns the static list of peers
func (s *StaticBootstrap) Discover(ctx context.Context) ([]string, error) {
	return s.peers, nil
}

// Register does nothing in static mode
func (s *StaticBootstrap) Register(ctx context.Context, node *domain.Node) error {
	return nil
}

// Deregister does nothing in static mode
func (s *StaticBootstrap) Deregister(ctx context.Context, node *domain.Node) error {
	return nil
}
