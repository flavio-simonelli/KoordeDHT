package client

import (
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	ErrNoConnInPool = fmt.Errorf("clientpool: no connection in pool")
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
	selfId         domain.ID
	selfAddr       string
	lgr            logger.Logger
	mu             sync.Mutex
	clients        map[string]*refConn
	closed         bool          // indicates if the pool has been closed
	failureTimeout time.Duration // timeout for RPC calls (after which the server is considered unresponsive)
}

// New creates a new empty Pool. It accepts a list of functional options
// to configure the pool (logger).
func New(selfId domain.ID, selfAddr string, failTO time.Duration, opt ...Option) *Pool {
	p := &Pool{
		selfId:         selfId,
		selfAddr:       selfAddr,
		clients:        make(map[string]*refConn),
		lgr:            &logger.NopLogger{}, // default: no logging
		closed:         false,
		failureTimeout: failTO,
	}
	// Apply functional options
	for _, o := range opt {
		o(p)
	}
	return p
}

// FailureTimeout returns the default timeout for RPC calls.
func (p *Pool) FailureTimeout() time.Duration {
	return p.failureTimeout
}

// AddRef ensures that a gRPC connection to the given node exists in the pool.
// If the connection already exists, its reference count is incremented.
// If not, a new connection is created and tracked with an initial reference count of 1.
//
// This method should be called whenever a node is added to the RoutingTable
// (e.g., as successor or de Bruijn pointer or Predecessor).
func (p *Pool) AddRef(addr string) error {
	if addr == "" {
		return fmt.Errorf("clientpool: empty address")
	}
	if addr == p.selfAddr {
		return fmt.Errorf("clientpool: requested self address")
	}
	p.mu.Lock()
	if p.closed {
		return fmt.Errorf("clientpool: pool is closed")
	}
	// if connection already exists, increment refs and return
	if rc, ok := p.clients[addr]; ok {
		rc.refs++
		p.mu.Unlock()
		return nil
	}
	// otherwise create new connection
	conn, dialErr := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if dialErr != nil {
		p.mu.Unlock()
		return dialErr
	}
	p.clients[addr] = &refConn{conn: conn, refs: 1}
	p.mu.Unlock()
	p.lgr.Debug("Pool: new connection added", logger.F("addr", addr))
	return nil
}

// GetFromPool returns a gRPC client backed by a pooled connection.
// The connection is managed by the pool and MUST NOT be closed by the caller.
func (p *Pool) GetFromPool(addr string) (dhtv1.DHTClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("clientpool: empty address")
	}
	if addr == p.selfAddr {
		return nil, fmt.Errorf("clientpool: requested self address")
	}
	p.mu.Lock()
	if p.closed {
		return nil, fmt.Errorf("clientpool: pool is closed")
	}
	rc, ok := p.clients[addr]
	p.mu.Unlock()
	if !ok {
		return nil, ErrNoConnInPool
	}
	return dhtv1.NewDHTClient(rc.conn), nil
}

// DialEphemeral creates a new one-shot gRPC connection to the given address.
// The connection is NOT added to the pool; the caller is responsible for closing it.
func (p *Pool) DialEphemeral(addr string) (dhtv1.DHTClient, *grpc.ClientConn, error) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return nil, nil, fmt.Errorf("clientpool: pool is closed")
	}
	p.mu.Unlock()
	if addr == "" {
		return nil, nil, fmt.Errorf("clientpool: empty address")
	}
	if addr == p.selfAddr {
		return nil, nil, fmt.Errorf("clientpool: requested self address")
	}
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // plaintext, no TLS
	)
	if err != nil {
		p.lgr.Error("DialEphemeral: failed to dial",
			logger.F("addr", addr),
			logger.F("err", err),
		)
		return nil, nil, fmt.Errorf("clientpool: failed to dial %s: %w", addr, err)
	}
	p.lgr.Debug("DialEphemeral: connection created",
		logger.F("addr", addr),
	)
	return dhtv1.NewDHTClient(conn), conn, nil
}

// Release decreases the reference count for the given node.
// When the reference count reaches zero, the underlying gRPC
// connection is closed and removed from the pool.
//
// This method must be called whenever a node is removed from
// the RoutingTable (e.g., no longer a successor or de Bruijn pointer).
func (p *Pool) Release(addr string) error {
	if addr == "" {
		return fmt.Errorf("clientpool: empty address")
	}
	var rc *refConn
	var refs int
	var ok bool
	p.mu.Lock()
	if p.closed {
		return fmt.Errorf("clientpool: pool is closed")
	}
	rc, ok = p.clients[addr]
	if ok {
		rc.refs--
		refs = rc.refs
		if refs <= 0 {
			delete(p.clients, addr)
		}
	}
	p.mu.Unlock()
	if !ok || refs > 0 {
		return nil
	}
	// if refs == 0, close the connection
	if err := rc.conn.Close(); err != nil {
		p.lgr.Error("Pool: failed to close connection",
			logger.F("addr", addr),
			logger.F("err", err),
		)
		return fmt.Errorf("clientpool: failed to close connection for node %s: %w", addr, err)
	}
	p.lgr.Debug("Pool: connection removed", logger.F("addr", addr))
	return nil
}

// Close shuts down all active gRPC connections and clears the pool.
//
// This method is safe to call multiple times; only the first call
// has an effect. Subsequent calls return immediately without error.
//
// Close ensures that all client connections are closed and the pool
// is marked as unusable. If one or more connections fail to close,
// the first encountered error is returned, but all connections are
// attempted regardless.
func (p *Pool) Close() error {
	p.mu.Lock()
	if p.closed {
		// Pool already closed → nothing to do
		p.mu.Unlock()
		return nil
	}
	p.closed = true

	// Take a snapshot of current connections
	conns := make([]*grpc.ClientConn, 0, len(p.clients))
	for _, rc := range p.clients {
		conns = append(conns, rc.conn)
	}

	// Reset the pool map so new operations see an empty pool
	p.clients = make(map[string]*refConn)
	p.mu.Unlock()

	var firstErr error
	for _, conn := range conns {
		// Attempt to close each connection
		if err := conn.Close(); err != nil {
			// Record the first error but keep closing the rest
			if firstErr == nil {
				firstErr = err
			}
			p.lgr.Error("Pool: failed to close connection", logger.F("err", err))
		}
	}
	return firstErr
}

// DebugLog emits a structured DEBUG-level log with a snapshot of the client pool.
//
// The log entry includes all active connections with their reference counts.
// If the pool is empty, the snapshot will contain an empty slice.
func (p *Pool) DebugLog() {
	p.mu.Lock()
	if p.closed {
		p.lgr.Debug("ClientPool snapshot: pool is closed")
		p.mu.Unlock()
		return
	}
	snapshot := make(map[string]int, len(p.clients))
	for addr, rc := range p.clients {
		snapshot[addr] = rc.refs
	}
	p.mu.Unlock()
	entries := make([]map[string]any, 0, len(snapshot))
	for addr, refs := range snapshot {
		entries = append(entries, map[string]any{
			"addr": addr,
			"refs": refs,
		})
	}
	p.lgr.Debug("ClientPool snapshot", logger.F("entries", entries))
}
