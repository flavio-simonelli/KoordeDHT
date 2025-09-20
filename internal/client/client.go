package client

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	dhtv1 "KoordeDHT/internal/api/dht/v1"
)

// Manager gestisce connessioni gRPC riusabili per indirizzo.
type Manager struct {
	mu          sync.RWMutex
	conns       map[string]*connEntry
	dialTimeout time.Duration
	idleTTL     time.Duration
	stopCh      chan struct{}
}

type connEntry struct {
	conn     *grpc.ClientConn
	lastUsed time.Time
}

// New crea un manager minimale.
// dialTimeout: timeout per il dial di nuova connessione.
// idleTTL: se >0, le connessioni inattive da almeno idleTTL vengono chiuse periodicamente.
func New(dialTimeout, idleTTL time.Duration) *Manager {
	m := &Manager{
		conns:       make(map[string]*connEntry),
		dialTimeout: dialTimeout,
		idleTTL:     idleTTL,
		stopCh:      make(chan struct{}),
	}
	if idleTTL > 0 {
		go m.evictLoop()
	}
	return m
}

// Close chiude tutte le connessioni e ferma l'evict loop.
func (m *Manager) Close() {
	close(m.stopCh)
	m.mu.Lock()
	defer m.mu.Unlock()
	for addr, ce := range m.conns {
		_ = ce.conn.Close()
		delete(m.conns, addr)
	}
}

// Do esegue fn con un client tipizzato verso addr.
// Crea la connessione se non esiste, poi la riusa.
func (m *Manager) Do(ctx context.Context, addr string, fn func(client dhtv1.DHTClient) error) error {
	conn, err := m.getConn(ctx, addr)
	if err != nil {
		return err
	}
	client := dhtv1.NewDHTClient(conn)
	return fn(client)
}

// getConn ritorna (o crea) una *grpc.ClientConn riusabile per addr.
func (m *Manager) getConn(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	// fast path: già in cache
	m.mu.RLock()
	if ce, ok := m.conns[addr]; ok {
		ce.lastUsed = time.Now()
		c := ce.conn
		m.mu.RUnlock()
		return c, nil
	}
	m.mu.RUnlock()

	// slow path: dial e memorizza
	m.mu.Lock()
	defer m.mu.Unlock()
	// ricontrollo: qualcun altro potrebbe averla creata
	if ce, ok := m.conns[addr]; ok {
		ce.lastUsed = time.Now()
		return ce.conn, nil
	}

	ctxDial, cancel := context.WithTimeout(ctx, m.dialTimeout)
	defer cancel()
	conn, err := grpc.DialContext(
		ctxDial,
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	m.conns[addr] = &connEntry{conn: conn, lastUsed: time.Now()}
	return conn, nil
}

// --- eviction minimale ---

func (m *Manager) evictLoop() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-t.C:
			m.evictIdle()
		}
	}
}

func (m *Manager) evictIdle() {
	if m.idleTTL <= 0 {
		return
	}
	now := time.Now()
	var toClose []*grpc.ClientConn

	m.mu.Lock()
	for addr, ce := range m.conns {
		if now.Sub(ce.lastUsed) >= m.idleTTL {
			toClose = append(toClose, ce.conn)
			delete(m.conns, addr)
		}
	}
	m.mu.Unlock()

	for _, c := range toClose {
		_ = c.Close()
	}
}
