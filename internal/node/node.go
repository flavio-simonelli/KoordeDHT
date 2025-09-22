package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/storage"
)

type Node struct {
	lgr logger.Logger
	rt  *routingtable.RoutingTable
	s   storage.Storage
	cp  *client.ClientPool
}

func New(self domain.Node, idBits, degree int, opts ...Option) (*Node, error) {
	n := &Node{
		lgr: &logger.NopLogger{},
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(n)
	}
	// inizializza la routing table
	rout, err := routingtable.New(self, idBits, degree, routingtable.WithLogger(n.lgr.Named("routingtable")))
	if err != nil {
		return nil, err
	}
	n.rt = rout
	// inizializza lo storage
	n.s = storage.NewMemoryStorage(n.lgr.Named("storage"))
	// inizializza il client pool
	n.cp = client.NewClientPool(n.lgr.Named("clientpool"))

	return n, nil
}

func (n *Node) Join(bootstrapAddr string) error {
	// richiesta al nodo di bootstrap per trovare il mio successore
	succ, err := n.cp.FindSuccessorInit(n.rt.Self().ID, bootstrapAddr)
	if err != nil {
		return err
	}
	n.lgr.Info("Il mio possibile successore è", logger.FNode("successor", succ))
	// contatto il mio possibile successore per ottenere il suo predecessore
	pred, err := n.cp.GetPredecessor(succ.Addr)
	if err != nil {
		return err
	}
	n.lgr.Info("Il predecessore del mio successore è", logger.FNode("predecessor", pred))
	// contatto il mio successore per aggiornare il suo predecessore a me
	err = n.cp.Notify(n.rt.Self(), succ.Addr)
	if err != nil {
		return err
	}
	// aggiorno la mia routing table
	n.rt.SetPredecessor(pred)
	n.rt.SetSuccessor(succ)
	// inizializzare la routing table con i DebrujinLinks
	n.FixDebruijnLinks()
	return nil
}
