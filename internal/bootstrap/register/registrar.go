package register

import (
	"context"
)

// Registrar is a generic interface for node registration backends (Route53, etcd, ...).
type Registrar interface {
	RegisterNode(ctx context.Context, nodeID, targetHost string, port int) error
	DeregisterNode(ctx context.Context, nodeID, targetHost string, port int) error
	RenewNode(ctx context.Context, nodeID, targetHost string, port int) error
	Close() error
}
