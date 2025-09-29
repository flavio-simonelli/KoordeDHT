package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/routingtable"
	"KoordeDHT/internal/storage"
	"context"
	"fmt"

	"google.golang.org/grpc"
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
		_ = n.cp.AddRef(pred.Addr)
		n.rt.SetPredecessor(pred)
	}
	_ = n.cp.AddRef(succ.Addr)
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
// The procedure notifies the successor about departure and attempts
// to transfer all resources currently stored at this node.
//
// Behavior:
//   - If this is the only node in the ring, the leave is a no-op.
//   - Otherwise:
//     1. Notify the successor of the departure.
//     2. Attempt to transfer all resources to the immediate successor.
//     3. If some resources cannot be transferred, resolve their
//     responsible node via FindSuccessor and retry individually.
//   - Logs INFO on successful transfers, WARN/ERROR on failures.
//
// Returns:
//   - nil if the leave was completed successfully (resources either
//     transferred or retried).
//   - error if resource transfer ultimately fails for some keys.
func (n *Node) Leave() error {
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()

	// Case: single node in the ring
	if succ == nil || succ.ID.Equal(self.ID) {
		n.lgr.Warn("leave: single node in DHT, no need to notify others", logger.FNode("self", self))
		return nil
	}

	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		return fmt.Errorf("leave: failed to get client for successor %s: %w", succ.Addr, err)
	}

	// Notify successor of departure (best-effort)
	{
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		if err := client.Leave(ctx, cli, self); err != nil {
			n.lgr.Error("leave: failed to notify successor", logger.F("successor", succ.Addr), logger.F("err", err))
			// Continue anyway with resource transfer
		}
		cancel()
	}

	// Attempt bulk transfer to successor
	data := n.s.All()
	if len(data) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		failed, err := client.StoreRemote(ctx, cli, data)
		cancel()
		if err != nil {
			n.lgr.Warn("Leave: bulk transfer to successor failed, retrying individually",
				logger.F("total", len(data)), logger.F("err", err))
			failed = data // treat all as failed
		}

		// Retry individually for any failed resources
		for _, res := range failed {
			// Find the correct successor for this resource
			ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
			correctSucc, err := client.FindSuccessorStart(ctx, cli, n.Space(), res.Key)
			cancel()
			if err != nil {
				n.lgr.Warn("Leave: failed to find responsible node for resource",
					logger.F("key", res.RawKey), logger.F("err", err))
				continue
			}
			if correctSucc == nil {
				n.lgr.Warn("Leave: no responsible node found for resource",
					logger.F("key", res.RawKey))
				continue
			}
			if correctSucc.ID.Equal(self.ID) {
				continue // the successor not keeps the resource, skip
			}
			cli2, err := n.cp.GetFromPool(correctSucc.Addr)
			var econn2 *grpc.ClientConn
			if err != nil {
				cli2, econn2, err = n.cp.DialEphemeral(correctSucc.Addr)
				if err != nil {
					n.lgr.Warn("Leave: failed to connect to responsible node",
						logger.F("key", res.RawKey), logger.FNode("responsible", correctSucc), logger.F("err", err))
					continue
				}
				defer econn2.Close()
			}

			sres := []domain.Resource{res}
			ctx, cancel = context.WithTimeout(context.Background(), n.cp.FailureTimeout())
			_, err = client.StoreRemote(ctx, cli2, sres)
			cancel()
			if err != nil {
				n.lgr.Warn("Leave: failed to transfer resource during retry",
					logger.F("key", res.RawKey), logger.FNode("responsible", correctSucc), logger.F("err", err))
				continue
			}

			n.lgr.Info("Leave: resource transferred successfully during retry",
				logger.F("key", res.RawKey), logger.FNode("responsible", correctSucc))
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
	_ = n.Leave()
	if n.cp != nil {
		_ = n.cp.Close()
	}
	n.lgr.Info("node stopped gracefully")
}
