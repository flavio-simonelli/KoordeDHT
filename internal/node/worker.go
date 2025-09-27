package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"time"
)

// StartStabilizers runs periodic maintenance tasks for Koorde.
func (n *Node) StartStabilizers(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				n.lgr.Info("stabilizer stopped")
				return
			case <-ticker.C:
				n.stabilizeSuccessor() // keep successor pointer consistent
				n.fixSuccessorList()   // refresh successor list
				n.checkPredecessor()   // remove dead predecessor if needed
				n.fixDeBruijn()        // maintain de Bruijn pointer
				n.printRoutingTable()
				n.printClientPoolStats()
				n.printStorageStats()
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

func (n *Node) printRoutingTable() {
	n.rt.DebugLog()
}

// stabilizeSuccessor checks whether our successor is still valid
// and updates it if its predecessor is a better candidate.
func (n *Node) stabilizeSuccessor() {
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Error("IMPOSSIBLE -> stabilize: successor nil")
		return
	}
	// Ask successor for its predecessor
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	pred, err := n.cp.GetPredecessor(ctx, succ.Addr)
	if err != nil {
		n.lgr.Warn("stabilize: could not contact successor",
			logger.FNode("succ", succ),
			logger.F("err", err))
		// Promote next available successor from the list
		promoted := false
		for i := 1; i < n.rt.SuccListSize(); i++ {
			candidate := n.rt.GetSuccessor(i)
			if candidate == nil {
				continue
			}
			n.lgr.Debug("stabilize: promoting new successor",
				logger.FNode("old", succ),
				logger.FNode("new", candidate))

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
			// no candidate found
			// Release predecessor
			if pred := n.rt.GetPredecessor(); pred != nil {
				if err := n.cp.Release(pred.Addr); err != nil {
					n.lgr.Warn("stabilize: failed to release predecessor",
						logger.FNode("pred", pred), logger.F("err", err))
				}
			}
			// Release successor list
			for _, nd := range n.rt.SuccessorList() {
				if nd != nil {
					if err := n.cp.Release(nd.Addr); err != nil {
						n.lgr.Warn("stabilize: failed to release successor",
							logger.FNode("succ", nd), logger.F("err", err))
					}
				}
			}
			// Release de Bruijn list
			for _, nd := range n.rt.DeBruijnList() {
				if nd != nil {
					if err := n.cp.Release(nd.Addr); err != nil {
						n.lgr.Warn("stabilize: failed to release deBruijn entry",
							logger.FNode("node", nd), logger.F("err", err))
					}
				}
			}
			n.rt.InitSingleNode()
			return
		}
	}
	// If successor’s predecessor is closer, promote it
	if pred != nil && pred.ID.Between(self.ID, succ.ID) && !pred.ID.Equal(self.ID) {
		n.lgr.Debug("stabilize: successor updated",
			logger.FNode("old_successor", succ),
			logger.FNode("new_successor", pred))
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
	// Notify successor that we might be its predecessor
	ctx, cancel = context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	if err := n.cp.Notify(ctx, self, succ.Addr); err != nil {
		n.lgr.Warn("stabilize: notify failed",
			logger.FNode("succ", succ), logger.F("err", err))
	}
}

// fixSuccessorList refreshes the local successor list by asking
// the first successor for its list. It ensures reference counting
// by AddRef() on the new list before setting it, and Release() on
// the old list afterwards.
func (n *Node) fixSuccessorList() {
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Error("fixSuccessorList: no successor set")
		return
	}
	// Ask successor for its successor list
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	remoteList, err := n.cp.GetSuccessorList(ctx, succ.Addr)
	if err != nil {
		n.lgr.Warn("fixSuccessorList: could not fetch successor list",
			logger.FNode("succ", succ),
			logger.F("err", err))
		return
	}
	// Old list snapshot (for later release)
	oldList := n.rt.SuccessorList()
	oldSet := make(map[string]*domain.Node, len(oldList))
	for _, nd := range oldList {
		if nd != nil {
			oldSet[nd.Addr] = nd
		}
	}
	// Build new list
	size := n.rt.SuccListSize()
	newList := make([]*domain.Node, size)
	newList[0] = succ
	for i := 1; i < size; i++ {
		if i-1 < len(remoteList) {
			if remoteList[i-1] != nil && !remoteList[i-1].ID.Equal(n.rt.Self().ID) {
				newList[i] = remoteList[i-1]
			}
		}
	}
	newSet := make(map[string]*domain.Node, len(newList))
	for _, nd := range newList {
		if nd != nil {
			newSet[nd.Addr] = nd
		}
	}
	// addRef sui nuovi nodi
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

	// Release sui nodi rimossi
	for addr, nd := range oldSet {
		if _, ok := newSet[addr]; !ok {
			if err := n.cp.Release(addr); err != nil {
				n.lgr.Warn("fixSuccessorList: release failed",
					logger.FNode("node", nd), logger.F("err", err))
			}
		}
	}
}

// checkPredecessor verifies whether our predecessor is still alive.
// If it does not respond, we drop it.
func (n *Node) checkPredecessor() {
	pred := n.rt.GetPredecessor()
	if pred == nil || pred.ID.Equal(n.rt.Self().ID) {
		return
	}
	// Try a lightweight ping
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	if err := n.cp.Ping(ctx, pred.Addr); err != nil {
		n.lgr.Warn("checkPredecessor: predecessor unresponsive, clearing",
			logger.FNode("pred", pred),
			logger.F("err", err))
		// Release from client pool
		if err := n.cp.Release(pred.Addr); err != nil {
			n.lgr.Warn("checkPredecessor: failed to release predecessor from pool",
				logger.FNode("pred", pred),
				logger.F("err", err))
		}
		// Clear predecessor
		n.rt.SetPredecessor(nil)
	}
}

// fixDeBruijn refreshes the de Bruijn window.
// It finds the anchor (predecessor of k*m mod 2^b), updates digit 0,
// then fills the remaining digits using the anchor's successor list.
func (n *Node) fixDeBruijn() {
	self := n.rt.Self()
	// Compute target = (k * self.ID) mod 2^b
	target := n.rt.Space().MulKMod(self.ID)
	n.lgr.Debug("fixDeBruijn: checking target", logger.F("target", target.String()))
	// Lookup successor of target
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	succ, err := n.FindSuccessorInit(ctx, target)
	if err != nil || succ == nil {
		n.lgr.Warn("fixDeBruijn: could not find successor",
			logger.F("target", target.String()),
			logger.F("err", err))
		return
	}
	// Get predecessor of that successor (anchor)
	ctx, cancel = context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	anchor, err := n.cp.GetPredecessor(ctx, succ.Addr)
	if err != nil || anchor == nil {
		n.lgr.Warn("fixDeBruijn: could not get anchor predecessor",
			logger.FNode("succ", succ),
			logger.F("err", err))
		return
	}
	// Snapshot finestra attuale (oldSet)
	oldList := n.rt.DeBruijnList()
	oldSet := make(map[string]*domain.Node)
	for _, node := range oldList {
		if node != nil {
			oldSet[node.Addr] = node
		}
	}
	// Costruisci nuova finestra (newNodes + newSet)
	newNodes := make([]*domain.Node, n.rt.Space().GraphGrade)
	newNodes[0] = anchor
	ctx, cancel = context.WithTimeout(context.Background(), n.cp.Timeout())
	defer cancel()
	list, err := n.cp.GetSuccessorList(ctx, anchor.Addr)
	if err != nil {
		n.lgr.Warn("fixDeBruijn: could not fetch successor list from anchor",
			logger.FNode("anchor", anchor), logger.F("err", err))
		return
	}
	for i := 1; i < n.rt.Space().GraphGrade; i++ {
		if i-1 < len(list) {
			newNodes[i] = list[i-1]
		}
	}
	newSet := make(map[string]*domain.Node)
	for _, node := range newNodes {
		if node != nil {
			newSet[node.Addr] = node
		}
	}
	// AddRef nodi nuovi
	for addr, cand := range newSet {
		if _, ok := oldSet[addr]; !ok {
			if err := n.cp.AddRef(addr); err != nil {
				n.lgr.Warn("fixDeBruijn: failed to addref node",
					logger.FNode("node", cand), logger.F("err", err))
			}
		}
	}
	// aggiorna la finestra
	n.rt.SetDeBruijnList(newNodes)
	// Release nodi rimossi
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
