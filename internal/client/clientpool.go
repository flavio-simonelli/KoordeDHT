package client

import (
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// --------------------------------------
// refConn
// --------------------------------------

// refConn wraps a gRPC client connection with a simple reference counter.
// The reference counter tracks how many routing table entries (e.g. successor,
// de Bruijn pointer) currently rely on this connection. The connection is only
// closed when the reference count drops to zero.
type refConn struct {
	conn *grpc.ClientConn // active gRPC connection to the remote node
	refs int              // number of active references to this connection
}

// --------------------------------------
// ClientPool
// --------------------------------------

// Pool manages gRPC client connections to nodes present in the RoutingTable.
// It uses reference counting to avoid closing connections that are still in use
// (a node can appear in multiple roles, e.g., successor and de Bruijn pointer).
type Pool struct {
	lgr     logger.Logger
	mu      sync.Mutex
	clients map[string]*refConn
}

// New creates a new empty Pool. It accepts a list of functional options
// to configure the pool (logger).
func New(opt ...Option) *Pool {
	p := &Pool{
		clients: make(map[string]*refConn),
		lgr:     &logger.NopLogger{}, // default: no logging
	}
	// Apply functional options
	for _, o := range opt {
		o(p)
	}
	return p
}

// AddRef ensures that a gRPC connection to the given node exists in the pool.
// If the connection already exists, its reference count is incremented.
// If not, a new connection is created and tracked with an initial reference count of 1.
//
// This method should be called whenever a node is added to the RoutingTable
// (e.g., as successor or de Bruijn pointer).
func (p *Pool) AddRef(node *domain.Node) error {
	if node == nil {
		return fmt.Errorf("clientpool: node is nil")
	}
	addr := node.Addr
	p.mu.Lock()
	defer p.mu.Unlock()
	// If a connection already exists, just bump the refcount.
	if rc, ok := p.clients[addr]; ok {
		rc.refs++
		return nil
	}
	// Otherwise, establish a new gRPC connection.
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // plaintext, no TLS
	)
	if err != nil {
		return err
	}
	p.clients[addr] = &refConn{
		conn: conn,
		refs: 1,
	}
	return nil
}

// Get returns a gRPC client for the given node.
// It assumes the node is already tracked in the pool via AddRef.
// If the node is nil or the connection is missing, an error is returned.
func (p *Pool) Get(node *domain.Node) (dhtv1.DHTClient, error) {
	if node == nil {
		return nil, fmt.Errorf("clientpool: node is nil")
	}
	addr := node.Addr
	p.mu.Lock()
	defer p.mu.Unlock()
	rc, ok := p.clients[addr]
	if !ok {
		return nil, fmt.Errorf("clientpool: no connection found for node %s", addr)
	}
	return dhtv1.NewDHTClient(rc.conn), nil
}

// Release decreases the reference count for the given node.
// When the reference count reaches zero, the underlying gRPC
// connection is closed and removed from the pool.
//
// This method must be called whenever a node is removed from
// the RoutingTable (e.g., no longer a successor or de Bruijn pointer).
func (p *Pool) Release(node *domain.Node) error {
	if node == nil {
		return fmt.Errorf("clientpool: node is nil")
	}
	addr := node.Addr
	p.mu.Lock()
	defer p.mu.Unlock()
	rc, ok := p.clients[addr]
	if !ok {
		return fmt.Errorf("clientpool: no connection found for node %s", addr)
	}
	rc.refs--
	if rc.refs <= 0 {
		if err := rc.conn.Close(); err != nil {
			delete(p.clients, addr) // ensure cleanup anyway
			return fmt.Errorf("clientpool: failed to close connection for node %s: %w", addr, err)
		}
		delete(p.clients, addr)
	}
	return nil
}

// Close shuts down all active gRPC connections and clears the pool.
// This method is typically called during node shutdown to ensure
// that all resources are properly released.
//
// If one or more connections fail to close, the first error encountered
// is returned. All connections are attempted to be closed regardless.
func (p *Pool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for addr, rc := range p.clients {
		if err := rc.conn.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("clientpool: failed to close connection for node %s: %w", addr, err)
		}
		delete(p.clients, addr)
	}
	return firstErr
}
