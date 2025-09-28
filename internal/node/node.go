package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/domain"
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

// Join connects this node to an existing Koorde DHT using the given list of bootstrap peers.
// It attempts to contact each peer in order until one responds successfully to a FindSuccessorStart(selfID).
// Once a valid successor is found, the node initializes its routing table, successor list,
// and de Bruijn pointers. If all peers fail, the join returns an error.
//
// Parameters:
//   - peers:   slice of bootstrap peer addresses ("host:port")
//
// Returns:
//   - error: if no bootstrap peer responded successfully
func (n *Node) Join(peers []string) error {
	if len(peers) == 0 {
		return fmt.Errorf("join: no bootstrap peers provided")
	}
	self := n.rt.Self()
	var succ *domain.Node
	var lastErr error
	// Try each peer until one succeeds (RPC FindSuccessor for self.ID)
	for _, addr := range peers {
		if addr == self.Addr {
			continue // skip self
		}
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		cli, conn, err := n.cp.DialEphemeral(addr)
		if err != nil {
			lastErr = fmt.Errorf("join: failed to dial bootstrap %s: %w", addr, err)
			cancel()
			continue
		}
		succ, lastErr = client.FindSuccessorStart(ctx, cli, n.Space(), self.ID)
		cancel()
		conn.Close()
		if lastErr == nil && succ != nil {
			n.lgr.Info("join: candidate successor found",
				logger.F("bootstrap", addr),
				logger.FNode("successor", succ))
			break
		}
		if lastErr != nil {
			n.lgr.Warn("join: bootstrap attempt failed",
				logger.F("bootstrap", addr), logger.F("err", lastErr))
		} else {
			lastErr = fmt.Errorf("bootstrap %s returned nil successor", addr)
		}
	}

	if succ == nil {
		return fmt.Errorf("join: all bootstrap attempts failed: %w", lastErr)
	}

	// Ask successor for its predecessor
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
	cli, conn, err := n.cp.DialEphemeral(succ.Addr)
	if err != nil {
		cancel()
		return fmt.Errorf("join: failed to dial successor %s: %w", succ.Addr, err)
	}
	pred, err := client.GetPredecessor(ctx, cli, n.Space())
	cancel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("join: failed to get predecessor of successor %s: %w", succ.Addr, err)
	}
	if pred != nil {
		n.lgr.Info("join: successor has predecessor", logger.FNode("predecessor", pred))
	}

	// Notify successor that we may be its predecessor
	ctx, cancel = context.WithTimeout(context.Background(), n.cp.FailureTimeout())
	err = client.Notify(ctx, cli, self)
	cancel()
	conn.Close()
	if err != nil {
		return fmt.Errorf("join: failed to notify successor %s: %w", succ.Addr, err)
	}

	// Update local routing table (release old, set new)
	if pred != nil {
		n.cp.AddRef(pred.Addr)
		n.rt.SetPredecessor(pred)
	}
	n.cp.AddRef(succ.Addr)
	n.rt.SetSuccessor(0, succ)

	// Initialize successor list using the new successor
	n.fixSuccessorList()

	// Initialize de Bruijn pointers
	n.fixDeBruijn()

	n.lgr.Info("join: completed successfully",
		logger.FNode("self", self),
		logger.FNode("successor", succ))
	return nil
}

// CreateNewDHT initializes this node as the first member of a new Koorde DHT.
//
// In single-node mode, the routing table is set so that:
//   - The predecessor entry point to self.
//   - The first successor entry points to self.
//   - The first de Bruijn entry points to self.
//   - All other routing entries remain nil.
//
// This method must be called only once, when no bootstrap peers
// are available and the node is intended to start a brand new DHT ring.
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

	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		return fmt.Errorf("leave: failed to get client for successor %s: %w", succ.Addr, err)
	}

	// 1. Notifica al successore che sto lasciando
	{
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		if err := client.Leave(ctx, cli, self); err != nil {
			n.lgr.Error("leave: failed to notify successor", logger.F("successor", succ.Addr), logger.F("err", err))
			// Continue anyway with resource transfer
		}
		cancel()
	}

	// 2. Trasferimento risorse al successore
	data := n.s.All()
	if len(data) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		if err := client.StoreRemote(ctx, cli, data); err != nil {
			cancel()
			return fmt.Errorf("leave: failed to transfer %d resources to successor %s: %w", len(data), succ.Addr, err)
		}
		cancel()
		n.lgr.Info("leave: transferred resources to successor", logger.F("count", len(data)), logger.FNode("successor", succ))
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
