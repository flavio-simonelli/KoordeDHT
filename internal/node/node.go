package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/routingtable"
)

type Node struct {
	logger logger.Logger
	rt     *routingtable.RoutingTable
}

func New(self domain.Node, idBits, degree int, opts ...Option) (*Node, error) {
	n := &Node{
		logger: &logger.NopLogger{},
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(n)
	}
	// logger figlio con contesto del nodo (component, node_id, addr)
	rtLog := n.logger.With(
		logger.F("component", "routingtable"),
		logger.F("node_id", self.ID.ToHexString()),
		logger.F("addr", self.Addr),
	)
	// inizializza la routing table
	rout, err := routingtable.New(self, idBits, degree, routingtable.WithLogger(rtLog))
	if err != nil {
		return nil, err
	}
	n.rt = rout

	return n, nil
}
