package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"time"

	"google.golang.org/grpc"
)

// StartStabilizers runs periodic maintenance tasks for Koorde.
// It launches two independent loops:
//   - Chord-style stabilizers (successor/predecessor management) at chordInterval
//   - De Bruijn pointer maintenance at deBruijnInterval
//
// Both loops stop when ctx is canceled.
func (n *Node) StartStabilizers(ctx context.Context, chordInterval, deBruijnInterval, storageInterval time.Duration) {
	// Chord-style stabilizers
	go func() {
		ticker := time.NewTicker(chordInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				n.lgr.Info("chord stabilizers stopped")
				return
			case <-ticker.C:
				n.stabilizeSuccessor()
				n.fixSuccessorList()
				n.checkPredecessor()
			}
		}
	}()

	// De Bruijn stabilizer
	go func() {
		ticker := time.NewTicker(deBruijnInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				n.lgr.Info("de Bruijn stabilizer stopped")
				return
			case <-ticker.C:
				n.fixDeBruijn()
			}
		}
	}()

	// Storage maintenance
	go func() {
		ticker := time.NewTicker(storageInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				n.lgr.Info("storage maintenance stopped")
				return
			case <-ticker.C:
				n.resourceRepair(ctx)
			}
		}
	}()
}

// printStorageStats logs the current state of the local storage.
func (n *Node) printStorageStats() {
	n.s.DebugLog()
}

// printClientPoolStats logs the current state of the client pool.
func (n *Node) printClientPoolStats() {
	n.cp.DebugLog()
}

// printRoutingTable logs the current state of the routing table.
func (n *Node) printRoutingTable() {
	n.rt.DebugLog()
}

// resourceRepair performs one maintenance pass to ensure that all resources
// stored locally still belong to this node's primary ownership interval.
//
// Ownership (no replication):
//   - This node (self) owns keys in (pred, self].
//   - Any local resource whose key ∉ (pred, self] should be transferred
//     to the node that is currently responsible for it.
//
// Strategy:
//   - Fast check using the predecessor interval when available.
//   - Robust confirmation via a fresh FindSuccessor lookup before transferring,
//     so we do not rely solely on potentially stale predecessor information.
//
// Logging:
//   - WARN for lookup/transfer/delete failures.
//   - INFO for successful transfers.
//   - Keep logs minimal; this runs periodically.
func (n *Node) resourceRepair(ctx context.Context) {
	self := n.rt.Self()
	pred := n.rt.GetPredecessor()
	if pred == nil {
		// Without a successor, we cannot determine our responsibility interval.
		n.lgr.Warn("ResourceRepair: skipping pass, successor is nil")
		return
	}

	resources := n.s.Between(self.ID, pred.ID)
	if len(resources) == 0 {
		// No resources to check
		return
	}

	for _, res := range resources {

		// find current responsible node
		resp, err := n.FindSuccessorInit(ctx, res.Key)
		if err != nil || resp == nil {
			n.lgr.Warn("ResourceRepair: failed to find successor",
				logger.F("key", res.RawKey), logger.F("err", err))
			continue
		}
		if resp.ID.Equal(self.ID) {
			// still responsible
			continue
		}

		// transfer resource
		sres := []domain.Resource{res}
		cli, err := n.cp.GetFromPool(resp.Addr)
		var econn *grpc.ClientConn
		if err != nil {
			cli, econn, err = n.cp.DialEphemeral(resp.Addr)
			if err != nil {
				n.lgr.Warn("ResourceRepair: failed to connect to responsible node",
					logger.F("key", res.RawKey), logger.FNode("responsible", resp), logger.F("err", err))
				continue
			}
			defer econn.Close()
		}

		if _, err := client.StoreRemote(ctx, cli, sres); err != nil {
			n.lgr.Warn("ResourceRepair: failed to transfer resource",
				logger.F("key", res.RawKey), logger.FNode("responsible", resp), logger.F("err", err))
			continue
		}

		// delete local copy only if transfer succeeded
		if err := n.s.Delete(res.Key); err != nil {
			n.lgr.Warn("ResourceRepair: failed to delete resource after transfer",
				logger.F("key", res.RawKey), logger.F("err", err))
		} else {
			n.lgr.Info("ResourceRepair: resource transferred successfully",
				logger.F("key", res.RawKey), logger.FNode("responsible", resp))
		}
	}
}

// stabilizeSuccessor verifies that the current successor is alive and valid.
// If the successor is unresponsive, it tries to promote another candidate
// from the successor list. If no candidates are found, the node reverts to
// single-node mode. If the successor's predecessor is a better fit, the
// routing table is updated accordingly.
//
// The procedure is:
//  1. Query the current successor for its predecessor.
//  2. If the successor is unreachable, attempt to promote a candidate
//     from the successor list. If none is available, reset to single-node mode.
//  3. If the successor’s predecessor is closer, adopt it as the new successor.
//  4. Notify the successor that we may be its predecessor.
func (n *Node) stabilizeSuccessor() {
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Error("stabilize: successor is nil (invalid state)")
		return
	}

	// Step 1: ask successor for its predecessor
	var pred *domain.Node
	{
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		defer cancel()
		if succ.ID.Equal(self.ID) {
			pred = n.rt.GetPredecessor()
		} else {
			cli, err := n.cp.GetFromPool(succ.Addr)
			if err != nil {
				n.lgr.Warn("stabilize: failed to get client for successor",
					logger.FNode("succ", succ),
					logger.F("err", err))
				return
			}
			pred, err = client.GetPredecessor(ctx, cli, n.rt.Space())
			if err != nil {
				n.lgr.Warn("stabilize: could not get predecessor from successor",
					logger.FNode("succ", succ),
					logger.F("err", err))
			}
		}
	}

	// Step 2: if unreachable, promote candidate from successor list
	if pred == nil {
		n.lgr.Warn("stabilize: successor unresponsive, attempting promotion",
			logger.FNode("old_successor", succ))

		promoted := false
		for i := 1; i < n.Space().SuccListSize; i++ {
			candidate := n.rt.GetSuccessor(i)
			if candidate == nil {
				continue
			}
			n.rt.PromoteCandidate(i)
			if err := n.cp.Release(succ.Addr); err != nil {
				n.lgr.Warn("stabilize: failed to release old successor",
					logger.FNode("old", succ), logger.F("err", err))
			}
			succ = candidate
			promoted = true
			break
		}
		if !promoted {
			// No candidates found, reset to single-node mode
			n.lgr.Warn("stabilize: no candidates found, reverting to single-node mode")
			if pred := n.rt.GetPredecessor(); pred != nil {
				_ = n.cp.Release(pred.Addr)
			}
			for _, nd := range n.rt.SuccessorList() {
				if nd != nil {
					_ = n.cp.Release(nd.Addr)
				}
			}
			for _, nd := range n.rt.DeBruijnList() {
				if nd != nil {
					_ = n.cp.Release(nd.Addr)
				}
			}
			n.rt.InitSingleNode()
			return
		}
	}

	// Step 3: if predecessor is closer, adopt it as new successor
	if pred != nil && pred.ID.Between(self.ID, succ.ID) && !pred.ID.Equal(self.ID) {
		// AddRef new successor
		if err := n.cp.AddRef(pred.Addr); err != nil {
			n.lgr.Warn("stabilize: failed to add new successor to pool",
				logger.FNode("new", pred), logger.F("err", err))
		}
		// Update routing table
		n.rt.SetSuccessor(0, pred)
		// Release old successor
		if err := n.cp.Release(succ.Addr); err != nil {
			n.lgr.Warn("stabilize: failed to release old successor",
				logger.FNode("old", succ), logger.F("err", err))
		}
		succ = pred
	}

	// Step 4: notify successor
	{
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		defer cancel()

		if succ.ID.Equal(self.ID) {
			// If successor is self, no need to notify
			return
		}

		cli, err := n.cp.GetFromPool(succ.Addr)
		if err != nil {
			n.lgr.Error("stabilize: client for successor not found in pool",
				logger.FNode("succ", succ), logger.F("err", err))
			return
		}

		if err := client.Notify(ctx, cli, self); err != nil {
			n.lgr.Warn("stabilize: notify RPC failed",
				logger.FNode("succ", succ), logger.F("err", err))
		}
	}
}

// fixSuccessorList refreshes the local successor list by contacting
// the first successor. It maintains reference counts by AddRef() for
// new entries before installing them, and Release() for nodes that
// are no longer part of the list.
//
// The procedure is:
//  1. Fetch the successor list from the first successor.
//  2. Merge it into a new list of fixed size, always starting with self’s successor.
//  3. Update the routing table.
//  4. Adjust client pool references.
func (n *Node) fixSuccessorList() {
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Error("fixSuccessorList: no successor set")
		return
	}
	if succ.ID.Equal(n.rt.Self().ID) {
		// Single-node mode, nothing to do
		return
	}

	// Step 1: fetch successor list from first successor
	var remoteList []*domain.Node
	{
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		cli, err := n.cp.GetFromPool(succ.Addr)
		if err != nil {
			n.lgr.Error("fixSuccessorList: failed to get from pool",
				logger.FNode("succ", succ),
				logger.F("err", err))
			cancel()
			return
		}
		remoteList, err = client.GetSuccessorList(ctx, cli, n.rt.Space())
		cancel()
		if err != nil {
			n.lgr.Warn("fixSuccessorList: could not get successor list",
				logger.FNode("succ", succ),
				logger.F("err", err))
			return
		}
	}

	// Step 2: snapshot current list (for later release)
	oldList := n.rt.SuccessorList()
	oldSet := make(map[string]*domain.Node, len(oldList))
	for _, nd := range oldList {
		if nd != nil {
			oldSet[nd.Addr] = nd
		}
	}

	// Step 3: build new list (fixed size, first entry is successor)
	size := n.Space().SuccListSize
	newList := make([]*domain.Node, size)
	newList[0] = succ
	for i := 1; i < size; i++ {
		if i-1 < len(remoteList) {
			if remoteList[i-1] != nil {
				if remoteList[i-1].ID.Equal(n.rt.Self().ID) {
					break
				} else {
					newList[i] = remoteList[i-1]
				}
			}

		}
	}

	// Step 4: compute new set for reference management
	newSet := make(map[string]*domain.Node, len(newList))
	for _, nd := range newList {
		if nd != nil {
			newSet[nd.Addr] = nd
		}
	}

	// addRef new nodes
	for addr, nd := range newSet {
		if _, ok := oldSet[addr]; !ok {
			if err := n.cp.AddRef(addr); err != nil {
				n.lgr.Warn("fixSuccessorList: addref failed",
					logger.FNode("node", nd), logger.F("err", err))
			}
		}
	}

	// Replace in routing table
	n.rt.SetSuccessorList(newList)

	// Release removed nodes
	for addr, nd := range oldSet {
		if _, ok := newSet[addr]; !ok {
			if err := n.cp.Release(addr); err != nil {
				n.lgr.Warn("fixSuccessorList: release failed",
					logger.FNode("node", nd), logger.F("err", err))
			}
		}
	}
}

// checkPredecessor verifies whether the current predecessor is still alive.
// The method proceeds as follows:
//   - If no predecessor is set or the predecessor is self, it returns immediately.
//   - Otherwise, it tries to obtain a gRPC client for the predecessor from the pool.
//   - If the client cannot be retrieved or a Ping RPC fails, the predecessor is
//     considered dead: it is released from the pool and cleared in the routing table.
//
// Note: a failed notification or release does not stop the cleanup process;
// the predecessor pointer is always cleared in case of failure.
func (n *Node) checkPredecessor() {
	pred := n.rt.GetPredecessor()
	if pred == nil || pred.ID.Equal(n.rt.Self().ID) {
		return
	}

	// Acquire client connection from pool
	cli, err := n.cp.GetFromPool(pred.Addr)
	if err != nil {
		n.lgr.Warn("checkPredecessor: failed to get client for predecessor",
			logger.FNode("pred", pred),
			logger.F("err", err))
		// Without a client, assume predecessor is dead
		n.rt.SetPredecessor(nil)
		return
	}

	// Attempt a lightweight ping
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
	defer cancel()
	if err := client.Ping(ctx, cli); err != nil {
		n.lgr.Warn("checkPredecessor: predecessor unresponsive, clearing",
			logger.FNode("pred", pred),
			logger.F("err", err))

		// Release client from pool
		if err := n.cp.Release(pred.Addr); err != nil {
			n.lgr.Warn("checkPredecessor: failed to release predecessor from pool",
				logger.FNode("pred", pred),
				logger.F("err", err))
		}

		// Clear predecessor reference
		n.rt.SetPredecessor(nil)
	}
}

// fixDeBruijn refreshes the de Bruijn window for this node.
// The procedure is:
//  1. Compute the anchor as the predecessor of (k * self.ID) mod 2^b.
//  2. Set digit 0 of the de Bruijn window to the anchor.
//  3. Fill the remaining digits with entries from the anchor’s successor list.
//  4. Update the local routing table and adjust client pool references.
func (n *Node) fixDeBruijn() {
	self := n.rt.Self()
	// Step 1: compute target = (k * self.ID) mod 2^b
	target, err := n.rt.Space().MulKMod(self.ID)
	if err != nil {
		n.lgr.Error("fixDeBruijn: failed to compute target", logger.F("err", err))
		return
	}

	// Lookup successor of target
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
	succ, err := n.FindSuccessorInit(ctx, target)
	cancel()
	if err != nil || succ == nil {
		n.lgr.Warn("fixDeBruijn: could not find successor",
			logger.F("target", target.ToHexString(true)),
			logger.F("err", err))
		return
	}

	// Step 2: get anchor (predecessor of succ)
	var anchor *domain.Node
	{
		if succ.ID.Equal(self.ID) {
			anchor = n.rt.GetPredecessor()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
			cli, err := n.cp.GetFromPool(succ.Addr)
			if err != nil {
				ephCli, conn, err := n.cp.DialEphemeral(succ.Addr)
				if err != nil {
					n.lgr.Warn("fixDeBruijn: could not dial anchor successor",
						logger.FNode("succ", succ),
						logger.F("err", err))
					cancel()
					return
				}
				cli = ephCli
				defer conn.Close()
			}
			anchor, err = client.GetPredecessor(ctx, cli, n.rt.Space())
			cancel()
			if err != nil {
				n.lgr.Warn("fixDeBruijn: could not get the anchor",
					logger.FNode("succ", succ),
					logger.F("err", err))
				return
			}
		}
		if anchor == nil {
			n.lgr.Warn("fixDeBruijn: anchor is nil", logger.FNode("succ", succ))
			return
		}
	}

	// Snapshot current window
	oldList := n.rt.DeBruijnList()
	oldSet := make(map[string]*domain.Node)
	for _, node := range oldList {
		if node != nil {
			oldSet[node.Addr] = node
		}
	}

	// Step 3: build new window (digit 0 = anchor, others from anchor’s successor list)
	newNodes := make([]*domain.Node, n.rt.Space().GraphGrade)
	newNodes[0] = anchor

	var succList []*domain.Node
	{
		if anchor.ID.Equal(self.ID) {
			succList = n.rt.SuccessorList()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
			cli, err := n.cp.GetFromPool(anchor.Addr)
			if err != nil {
				ephCli, conn, err := n.cp.DialEphemeral(anchor.Addr)
				if err != nil {
					n.lgr.Warn("fixDeBruijn: could not dial anchor",
						logger.FNode("anchor", anchor), logger.F("err", err))
					cancel()
					return
				}
				cli = ephCli
				defer conn.Close()
			}
			succList, err = client.GetSuccessorList(ctx, cli, n.rt.Space())
			cancel()
			if err != nil {
				n.lgr.Warn("fixDeBruijn: could not get successor list from anchor",
					logger.FNode("anchor", anchor), logger.F("err", err))
				return
			}
		}
	}
	for i := 1; i < n.rt.Space().GraphGrade; i++ {
		if i-1 < len(succList) {
			newNodes[i] = succList[i-1]
		}
	}

	// Build set of new nodes
	newSet := make(map[string]*domain.Node)
	for _, node := range newNodes {
		if node != nil {
			newSet[node.Addr] = node
		}
	}

	// Step 4: update client pool references
	for addr, cand := range newSet {
		if _, ok := oldSet[addr]; !ok {
			if err := n.cp.AddRef(addr); err != nil {
				n.lgr.Warn("fixDeBruijn: failed to addref node",
					logger.FNode("node", cand), logger.F("err", err))
			}
		}
	}
	n.rt.SetDeBruijnList(newNodes)
	for addr, old := range oldSet {
		if _, ok := newSet[addr]; !ok {
			if err := n.cp.Release(addr); err != nil {
				n.lgr.Warn("fixDeBruijn: failed to release node",
					logger.FNode("old", old), logger.F("err", err))
			}
		}
	}

	n.lgr.Debug("fixDeBruijn: updated de Bruijn window",
		logger.F("degree", n.rt.Space().GraphGrade))
}
