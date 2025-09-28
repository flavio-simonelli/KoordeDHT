package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/storage"
	"context"
	"fmt"
)

type Node struct {
	lgr logger.Logger
	rt  *routingtable.RoutingTable
	s   *storage.Storage
	cp  *client.Pool
}

func New(rout *routingtable.RoutingTable, clientpool *client.Pool, storage *storage.Storage, opts ...Option) *Node {
	n := &Node{
		lgr: &logger.NopLogger{},
		rt:  rout,
		cp:  clientpool,
		s:   storage,
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(n)
	}
	return n
}

// TODO: da mettere nella join come codice
/*
// TryBootstrapJoin iterates over the given peers and attempts to join the DHT
// by calling FindSuccessorStart(selfID) on each peer using the client pool.
//
// Parameters:
//   - pool:    client connection pool
//   - selfID:  the local node identifier
//   - peers:   list of bootstrap peer addresses ("host:port")
//   - timeout: per-RPC timeout applied to each attempt
//
// Returns:
//   - *domain.Node: the successor node for selfID if join succeeded
//   - error: if no peer responded successfully
func (p *Pool) TryBootstrapJoin(selfID domain.ID, timeout time.Duration, peers []string) (*domain.Node, error) {
	var lastErr error

	for _, addr := range peers {
		// Create a context with timeout for this attempt
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		succ, err := p.FindSuccessorStart(ctx, selfID, addr)
		if err == nil && succ != nil {
			return succ, nil
		}
		if err != nil {
			lastErr = fmt.Errorf("FindSuccessorStart to %s failed: %w", addr, err)
		} else {
			lastErr = fmt.Errorf("peer %s returned nil successor", addr)
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no bootstrap peers provided")
	}
	return nil, fmt.Errorf("bootstrap join failed: %w", lastErr)
}
*/

// Join connects this node to an existing Koorde DHT using the given bootstrap peer.
// It sets predecessor/successor pointers, initializes the successor list, and sets de Bruijn links.
func (n *Node) Join(bootstrapAddr string) error {
	self := n.rt.Self()
	// 1. Ask bootstrap to find our successor
	if bootstrapAddr == self.Addr {
		return fmt.Errorf("join: bootstrap address cannot be self address %s", bootstrapAddr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	succ, err := n.cp.FindSuccessorStart(ctx, self.ID, bootstrapAddr)
	if err != nil {
		return fmt.Errorf("join: failed to find successor via bootstrap %s: %w", bootstrapAddr, err)
	}
	if succ == nil {
		return fmt.Errorf("join: bootstrap %s returned nil successor", bootstrapAddr)
	}
	n.lgr.Info("join: candidate successor found", logger.FNode("successor", succ))
	// 2. Ask successor for its predecessor
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	pred, err := n.cp.GetPredecessor(ctx, succ.Addr)
	if err != nil {
		return fmt.Errorf("join: failed to get predecessor of successor %s: %w", succ.Addr, err)
	}
	if pred != nil {
		n.lgr.Info("join: successor has predecessor", logger.FNode("predecessor", pred))
	}
	// 3. Notify successor that we may be its predecessor
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if err := n.cp.Notify(ctx, self, succ.Addr); err != nil {
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
	n.fixDeBruijn()

	n.lgr.Info("join: completed successfully",
		logger.FNode("self", self),
		logger.FNode("successor", succ))
	return nil
}

func (n *Node) CreateNewDHT() {
	n.rt.InitSingleNode()
}

// Leave gracefully removes the current node from the DHT.
// It notifies the successor and transfers all stored resources.
func (n *Node) Leave() error {
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()

	// Caso singolo nodo nell’anello
	if succ == nil || succ.ID.Equal(self.ID) {
		n.lgr.Warn("leave: single node in DHT, no need to notify others", logger.FNode("self", self))
		return nil
	}

	// 1. Notifica al successore che sto lasciando
	{
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		defer cancel()
		if err := n.cp.Leave(ctx, self, succ.Addr); err != nil {
			n.lgr.Error("leave: failed to notify successor", logger.F("err", err))
			// non ritorno subito → provo comunque a trasferire i dati
		}
	}

	// 2. Trasferimento risorse al successore
	data := n.s.All()
	if len(data) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		defer cancel()
		if err := n.cp.StoreRemote(ctx, data, succ.Addr); err != nil {
			return fmt.Errorf("leave: failed to transfer %d resources to successor %s: %w", len(data), succ.Addr, err)
		}
	}

	n.lgr.Info("leave: node has gracefully left the DHT", logger.FNode("self", self))
	return nil
}

// Stop releases all resources owned by the node.
// Should be called on shutdown.
func (n *Node) Stop() {
	if n == nil {
		return
	}
	// Example: close client pool, timers, background workers...
	if n.cp != nil {
		n.Leave()
		n.cp.Close()
	}
	// TODO: add other cleanup if needed
	n.lgr.Info("node stopped gracefully")
}
