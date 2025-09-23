package node

import (
	"KoordeDHT/internal/client"
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

func New(rout *routingtable.RoutingTable, opts ...Option) *Node {
	n := &Node{
		lgr: &logger.NopLogger{},
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(n)
	}
	n.rt = rout
	// inizializza lo storage
	n.s = storage.NewMemoryStorage(n.lgr.Named("storage"))
	// inizializza il client pool
	n.cp = client.NewClientPool(n.lgr.Named("clientpool"))
	return n
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
	n.rt.SetPredecessor(&pred)
	n.rt.SetSuccessor(0, &succ)
	// inizializza la successor list
	err := n.updateSuccessorList()
	if err != nil {
		return err
	}
	// inizializzare la routing table con i DebrujinLinks
	n.stabilizeDeBruijn()
	return nil
}

func (n *Node) CreateNewDHT() {
	n.rt.InitSingleNode()
}
