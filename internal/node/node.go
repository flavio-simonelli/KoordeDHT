package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/storage"
)

type Node struct {
	logger logger.Logger
	rt     *routingtable.RoutingTable
	s      storage.Storage
}

func New(self domain.Node, idBits, degree int, opts ...Option) (*Node, error) {
	n := &Node{
		logger: &logger.NopLogger{},
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(n)
	}
	// inizializza la routing table
	rout, err := routingtable.New(self, idBits, degree, routingtable.WithLogger(n.logger.With(logger.F("component", "routingtable"))))
	if err != nil {
		return nil, err
	}
	n.rt = rout
	// inizializza lo storage
	store := storage.NewMemoryStorage(n.logger.With(logger.F("component", "storage")))
	n.s = store

	return n, nil
}
