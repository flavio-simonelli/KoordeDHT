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
	selfId   domain.ID
	selfAddr string
	lgr      logger.Logger
	mu       sync.Mutex
	clients  map[string]*refConn
	timeout  time.Duration // default timeout for RPC calls
}

// New creates a new empty Pool. It accepts a list of functional options
// to configure the pool (logger).
func New(selfId domain.ID, selfAddr string, timeout time.Duration, opt ...Option) *Pool {
	p := &Pool{
		selfId:   selfId,
		selfAddr: selfAddr,
		clients:  make(map[string]*refConn),
		lgr:      &logger.NopLogger{}, // default: no logging
		timeout:  timeout,
	}
	// Apply functional options
	for _, o := range opt {
		o(p)
	}
	// Log pool creation
	p.lgr.Debug("client pool initialized")
	return p
}

// Timeout returns the default timeout for RPC calls.
func (p *Pool) Timeout() time.Duration {
	return p.timeout
}

// AddRef ensures that a gRPC connection to the given node exists in the pool.
// If the connection already exists, its reference count is incremented.
// If not, a new connection is created and tracked with an initial reference count of 1.
//
// This method should be called whenever a node is added to the RoutingTable
// (e.g., as successor or de Bruijn pointer).
func (p *Pool) AddRef(addr string) error {
	if addr == p.selfAddr {
		p.lgr.Warn("AddRef: attempted to AddRef to self, ignored",
			logger.F("addr", addr))
		return nil
	}
	p.mu.Lock()
	// Se già presente incremento refcount
	if rc, ok := p.clients[addr]; ok {
		rc.refs++
		refs := rc.refs
		p.mu.Unlock()

		p.lgr.Debug("AddRef: connection refcount incremented",
			logger.F("addr", addr),
			logger.F("refs", refs),
		)
		return nil
	}
	// Altrimenti creo la connessione dentro al lock (per evitare race)
	conn, dialErr := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if dialErr != nil {
		p.mu.Unlock()
		p.lgr.Error("AddRef: failed to establish connection",
			logger.F("addr", addr),
			logger.F("err", dialErr),
		)
		return dialErr
	}
	p.clients[addr] = &refConn{conn: conn, refs: 1}
	p.mu.Unlock()
	p.lgr.Debug("AddRef: new connection established",
		logger.F("addr", addr),
		logger.F("refs", 1),
	)
	return nil
}

// Get returns a gRPC client for the given node.
// If the connection exists in the pool, it reuses it.
// Otherwise, it creates a one-shot connection that is not tracked
// in the pool and must be closed by the caller after use.
func (p *Pool) Get(addr string) (dhtv1.DHTClient, error) {
	if addr == "" {
		p.lgr.Warn("Get: Get called with empty address")
		return nil, fmt.Errorf("clientpool: empty address")
	}
	p.mu.Lock()
	rc, ok := p.clients[addr]
	p.mu.Unlock()
	if ok {
		p.lgr.Debug("Get: reused pooled connection",
			logger.F("addr", addr),
			logger.F("refs", rc.refs),
		)
		// Connection managed by pool, caller must NOT close it
		return dhtv1.NewDHTClient(rc.conn), nil
	}
	// Create ephemeral connection (not pooled, caller must close it)
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // plaintext, no TLS
	)
	if err != nil {
		p.lgr.Error("Get: failed to create ephemeral connection",
			logger.F("addr", addr),
			logger.F("err", err),
		)
		return nil, fmt.Errorf("clientpool: failed to dial %s: %w", addr, err)
	}
	p.lgr.Debug("Get: ephemeral connection created",
		logger.F("addr", addr),
	)
	return dhtv1.NewDHTClient(conn), nil
}

// Release decreases the reference count for the given node.
// When the reference count reaches zero, the underlying gRPC
// connection is closed and removed from the pool.
//
// This method must be called whenever a node is removed from
// the RoutingTable (e.g., no longer a successor or de Bruijn pointer).
func (p *Pool) Release(addr string) error {
	if addr == p.selfAddr {
		p.lgr.Warn("Pool: attempted to Release self, ignored",
			logger.F("addr", addr))
		return nil
	}
	var rc *refConn
	var refs int
	var ok bool
	p.mu.Lock()
	rc, ok = p.clients[addr]
	if ok {
		rc.refs--
		refs = rc.refs
		if refs <= 0 {
			delete(p.clients, addr)
		}
	}
	p.mu.Unlock()
	// log
	if !ok {
		p.lgr.Warn("Pool: Release called for unknown connection",
			logger.F("addr", addr))
		return fmt.Errorf("clientpool: no connection found for node %s", addr)
	}
	if refs > 0 {
		p.lgr.Debug("Pool: connection refcount decremented",
			logger.F("addr", addr),
			logger.F("refs", refs),
		)
		return nil
	}
	// se refs == 0, chiudiamo la connessione
	if err := rc.conn.Close(); err != nil {
		p.lgr.Error("Pool: failed to close connection",
			logger.F("addr", addr),
			logger.F("err", err),
		)
		return fmt.Errorf("clientpool: failed to close connection for node %s: %w", addr, err)
	}
	p.lgr.Info("Pool: connection closed",
		logger.F("addr", addr),
	)
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
	// snapshot delle connessioni
	conns := make(map[string]*refConn, len(p.clients))
	for addr, rc := range p.clients {
		conns[addr] = rc
	}
	p.clients = make(map[string]*refConn) // reset
	p.mu.Unlock()

	var firstErr error
	for addr, rc := range conns {
		if err := rc.conn.Close(); err != nil {
			p.lgr.Error("Pool: failed to close connection",
				logger.F("addr", addr),
				logger.F("err", err),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("clientpool: failed to close connection for node %s: %w", addr, err)
			}
		} else {
			p.lgr.Info("Pool: connection closed",
				logger.F("addr", addr),
			)
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
