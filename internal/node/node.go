package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/storage"
	"fmt"
)

type Node struct {
	lgr logger.Logger
	rt  *routingtable.RoutingTable
	s   storage.Storage
	cp  *client.Pool
}

func New(rout *routingtable.RoutingTable, clientpool *client.Pool, opts ...Option) *Node {
	n := &Node{
		lgr: &logger.NopLogger{},
		rt:  rout,
		cp:  clientpool,
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// Join connects this node to an existing Koorde ring using the given bootstrap peer.
// It sets predecessor/successor pointers, initializes the successor list,
// and refreshes de Bruijn links.
func (n *Node) Join(bootstrapAddr string) error {
	self := n.rt.Self()
	// 1. Ask bootstrap to find our successor
	if bootstrapAddr == self.Addr {
		return fmt.Errorf("join: bootstrap address cannot be self address %s", bootstrapAddr)
	}
	succ, err := n.cp.FindSuccessorStart(self.ID, bootstrapAddr)
	if err != nil {
		return fmt.Errorf("join: failed to find successor via bootstrap %s: %w", bootstrapAddr, err)
	}
	if succ == nil {
		return fmt.Errorf("join: bootstrap %s returned nil successor", bootstrapAddr)
	}
	n.lgr.Info("join: candidate successor found", logger.FNode("successor", *succ))
	// 2. Ask successor for its predecessor
	pred, err := n.cp.GetPredecessor(succ.Addr)
	if err != nil {
		return fmt.Errorf("join: failed to get predecessor of successor %s: %w", succ.Addr, err)
	}
	if pred != nil {
		n.lgr.Info("join: successor has predecessor", logger.FNode("predecessor", *pred))
	}
	// 3. Notify successor that we may be its predecessor
	if err := n.cp.Notify(self, succ.Addr); err != nil {
		return fmt.Errorf("join: failed to notify successor %s: %w", succ.Addr, err)
	}
	// 4. Update local routing table (release old, set new)
	n.cp.AddRef(pred.Addr)
	n.rt.SetPredecessor(pred)
	n.cp.AddRef(succ.Addr)
	n.rt.SetSuccessor(0, succ)

	// 5. Initialize successor list using the new successor
	n.fixSuccessorList()

	// 6. Initialize de Bruijn pointers
	//n.fixDeBruijn()

	n.lgr.Info("join: completed successfully",
		logger.FNode("self", *self),
		logger.FNode("successor", *succ))
	return nil
}

func (n *Node) CreateNewDHT() {
	n.rt.InitSingleNode()
}

// Stop releases all resources owned by the node.
// Should be called on shutdown.
func (n *Node) Stop() {
	if n == nil {
		return
	}
	// Example: close client pool, timers, background workers...
	if n.cp != nil {
		n.cp.Close()
	}
	// TODO: add other cleanup if needed
	n.lgr.Info("node stopped gracefully")
}
