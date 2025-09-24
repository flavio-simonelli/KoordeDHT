package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"fmt"
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
			}
		}
	}()
}

func (n *Node) printRoutingTable() {
	fmt.Println(n.rt.DebugString())
}

// stabilizeSuccessor checks whether our successor is still valid
// and updates it if its predecessor is a better candidate.
func (n *Node) stabilizeSuccessor() {
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Warn("IMPOSSIBLE -> stabilize: successor nil")
		return
	}
	// Ask successor for its predecessor
	pred, err := n.cp.GetPredecessor(succ.Addr)
	if err != nil {
		n.lgr.Warn("stabilize: could not contact successor",
			logger.FNode("succ", *succ),
			logger.F("err", err))

		// Promote next available successor from the list
		promoted := false
		for i := 1; i < n.rt.SuccListSize(); i++ {
			candidate := n.rt.GetSuccessor(i)
			if candidate == nil {
				continue
			}
			n.lgr.Info("stabilize: promoting new successor",
				logger.FNode("old", *succ),
				logger.FNode("new", *candidate))

			n.rt.PromoteCandidate(i)
			succ = candidate
			promoted = true
			break
		}
		if !promoted {
			// no valid candidate found
			n.rt.InitSingleNode()
			return
		}
	}
	// If successor’s predecessor is closer, promote it
	if pred != nil && pred.ID.Between(self.ID, succ.ID) && !pred.ID.Equal(self.ID) {
		n.lgr.Info("stabilize: successor updated",
			logger.FNode("old_successor", *succ),
			logger.FNode("new_successor", *pred))
		n.rt.SetSuccessor(0, pred)
		succ = pred
	}
	// Notify successor that we might be its predecessor
	if err := n.cp.Notify(self, succ.Addr); err != nil {
		n.lgr.Warn("stabilize: notify failed",
			logger.FNode("succ", *succ), logger.F("err", err))
	}
}

// fixSuccessorList refreshes the local successor list by asking
// the first successor for its list. It ensures reference counting
// by AddRef() on the new list before setting it, and Release() on
// the old list afterwards.
func (n *Node) fixSuccessorList() {
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Warn("fixSuccessorList: no successor set")
		return
	}
	// Ask successor for its successor list
	remoteList, err := n.cp.GetSuccessorList(succ.Addr)
	if err != nil {
		n.lgr.Warn("fixSuccessorList: could not fetch successor list",
			logger.FNode("succ", *succ),
			logger.F("err", err))
		return
	}
	// Old list snapshot (for later release)
	oldList := n.rt.SuccessorList()
	// Build new list
	newList := make([]*domain.Node, n.rt.SuccListSize())
	newList[0] = succ
	for i := 1; i < n.rt.SuccListSize(); i++ {
		if i-1 < len(remoteList) {
			if remoteList[i-1] != nil && !remoteList[i-1].ID.Equal(succ.ID) {
				newList[i] = remoteList[i-1]
			}
		}
	}
	// AddRef on all new nodes
	for _, node := range newList {
		if node != nil {
			_ = n.cp.AddRef(node.Addr)
		}
	}
	// Replace in routing table
	n.rt.SetSuccessorList(newList)
	// Release all old nodes
	for _, node := range oldList {
		if node != nil {
			_ = n.cp.Release(node.Addr)
		}
	}
	n.lgr.Debug("fixSuccessorList: updated",
		logger.F("count", len(newList)))
}

// checkPredecessor verifies whether our predecessor is still alive.
// If it does not respond, we drop it.
func (n *Node) checkPredecessor() {
	pred := n.rt.GetPredecessor()
	if pred == nil || pred.ID.Equal(n.rt.Self().ID) {
		return
	}
	// Try a lightweight ping
	if err := n.cp.Ping(pred.Addr); err != nil {
		n.lgr.Warn("checkPredecessor: predecessor unresponsive, clearing",
			logger.FNode("pred", *pred),
			logger.F("err", err))
		n.rt.SetPredecessor(nil)
	}
}

// fixDeBruijn refreshes the de Bruijn window.
// It finds the anchor (predecessor of k*m mod 2^b), updates digit 0,
// then fills the remaining digits using the anchor's successor list.
func (n *Node) fixDeBruijn() {
	self := n.rt.Self()

	// 1. Compute target = (k * self.ID) mod 2^b
	target := n.rt.Space().MulKMod(self.ID)
	n.lgr.Info("fixDeBruijn: checking target", logger.F("target", target.String()))

	// 2. Lookup successor of target
	succ, err := n.FindSuccessorInit(context.Background(), target)
	if err != nil || succ == nil {
		n.lgr.Warn("fixDeBruijn: could not find successor",
			logger.F("target", target.String()),
			logger.F("err", err))
		return
	}

	// 3. Get predecessor of that successor (anchor)
	anchor, err := n.cp.GetPredecessor(succ.Addr)
	if err != nil || anchor == nil {
		n.lgr.Warn("fixDeBruijn: could not get anchor predecessor",
			logger.FNode("succ", *succ),
			logger.F("err", err))
		return
	}

	// Old anchor to release later if changed
	oldAnchor := n.rt.GetDeBruijn(0)
	if oldAnchor != nil && !oldAnchor.ID.Equal(anchor.ID) {
		if err := n.cp.Release(oldAnchor.Addr); err != nil {
			n.lgr.Warn("fixDeBruijn: failed to release old anchor",
				logger.FNode("old", *oldAnchor), logger.F("err", err))
		}
	}

	// AddRef on new anchor
	if err := n.cp.AddRef(anchor.Addr); err != nil {
		n.lgr.Warn("fixDeBruijn: failed to addref anchor",
			logger.FNode("anchor", *anchor), logger.F("err", err))
	}

	// 4. Set anchor at position 0
	n.rt.SetDeBruijn(0, anchor)

	// 5. Get successor list of anchor to populate remaining digits
	list, err := n.cp.GetSuccessorList(anchor.Addr)
	if err != nil {
		n.lgr.Warn("fixDeBruijn: could not fetch successor list from anchor",
			logger.FNode("anchor", *anchor), logger.F("err", err))
		return
	}

	// Fill positions 1..k-1 with successors of the anchor
	for i := 1; i < n.rt.Space().GraphGrade && i-1 < len(list); i++ {
		cand := list[i-1]
		if cand == nil {
			continue
		}

		old := n.rt.GetDeBruijn(i)
		if old != nil && !old.ID.Equal(cand.ID) {
			if err := n.cp.Release(old.Addr); err != nil {
				n.lgr.Warn("fixDeBruijn: failed to release old de Bruijn pointer",
					logger.FNode("old", *old), logger.F("err", err))
			}
		}

		if err := n.cp.AddRef(cand.Addr); err != nil {
			n.lgr.Warn("fixDeBruijn: failed to addref de Bruijn pointer",
				logger.FNode("node", *cand), logger.F("err", err))
		}
		n.rt.SetDeBruijn(i, cand)
	}

	n.lgr.Debug("fixDeBruijn: updated de Bruijn window",
		logger.F("degree", n.rt.Space().GraphGrade))
}
