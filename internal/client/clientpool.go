package client

import (
	"KoordeDHT/internal/logger"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientPool gestisce connessioni gRPC client riutilizzabili verso nodi della DHT.
type ClientPool struct {
	lgr           logger.Logger
	mu            sync.RWMutex
	conns         map[string]*grpc.ClientConn
	configOptions []grpc.DialOption
}

func NewClientPool(logger logger.Logger, opts ...grpc.DialOption) *ClientPool {
	if len(opts) == 0 {
		// Default: connessioni insicure (utile per test/ambienti trusted)
		opts = []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	}
	return &ClientPool{
		lgr:           logger,
		conns:         make(map[string]*grpc.ClientConn),
		configOptions: opts,
	}
}

// GetConn ritorna una connessione gRPC verso l'indirizzo addr.
// Se non esiste già una connessione attiva, ne viene creata una nuova.
func (cp *ClientPool) GetConn(addr string) (*grpc.ClientConn, error) {
	cp.mu.RLock()
	conn, exists := cp.conns[addr]
	cp.mu.RUnlock()
	if exists {
		return conn, nil
	}

	cp.mu.Lock()
	// Controlla di nuovo per evitare condizioni di gara
	if conn, exists = cp.conns[addr]; exists {
		cp.mu.Unlock()
		return conn, nil
	}
	newConn, err := grpc.NewClient(addr, cp.configOptions...)
	if err != nil {
		cp.mu.Unlock()
		return nil, err
	}
	cp.conns[addr] = newConn
	cp.mu.Unlock()
	cp.lgr.Info("Nuova connessione gRPC creata", logger.F("addr", addr))
	return newConn, nil
}

// CloseConn chiude la connessione gRPC verso l'indirizzo addr e la rimuove dal pool.
func (cp *ClientPool) CloseConn(addr string) error {
	cp.mu.Lock()
	conn, exists := cp.conns[addr]
	if !exists {
		cp.mu.Unlock()
		return nil // Nessuna connessione da chiudere
	}
	err := conn.Close()
	if err != nil {
		cp.mu.Unlock()
		return err
	}
	delete(cp.conns, addr)
	cp.mu.Unlock()
	cp.lgr.Info("Connessione gRPC chiusa", logger.F("addr", addr))
	return nil
}

// CloseAll chiude tutte le connessioni gRPC nel pool.
func (cp *ClientPool) CloseAll() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	for addr, conn := range cp.conns {
		err := conn.Close()
		if err != nil {
			return err
		}
		delete(cp.conns, addr)
		cp.lgr.Info("Connessione gRPC chiusa", logger.F("addr", addr))
	}
	cp.lgr.Info("ClientPool chiuso, tutte le connessioni gRPC sono state chiuse")
	return nil
}
