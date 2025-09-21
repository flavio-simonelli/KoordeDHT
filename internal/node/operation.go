package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
)

func (n *Node) FindSuccessor(target domain.ID) (domain.Node, error) {
	next, final := n.rt.FindSuccessor(target)
	if final {
		n.lgr.Info("FindSuccessor: target found in routing table", logger.F("target", target.ToHexString()), logger.FNode("successor", next))
		return next, nil
	}
	n.lgr.Info("FindSuccessor: target not found in routing table, querying next node", logger.F("target", target.ToHexString()), logger.FNode("next", next))
	return n.cp.FindSuccessor(target, next.Addr)
}

func (n *Node) GetPredecessor() domain.Node {
	pred := n.rt.Predecessor()
	n.lgr.Info("GetPredecessor", logger.FNode("predecessor", pred))
	return pred
}

func (n *Node) Notify(m domain.Node) {
	self := n.rt.Self()
	pred := n.rt.Predecessor()
	// se non ho predecessore, o m è tra (pred, self) → aggiorno
	if pred.ID.Equal(self.ID) || m.ID.InOO(pred.ID, self.ID) {
		n.lgr.Info("Notify: updating predecessor",
			logger.FNode("old_predecessor", pred),
			logger.FNode("new_predecessor", m),
		)
		n.rt.SetPredecessor(m)
	} else {
		// altrimenti ignoro
		n.lgr.Debug("Notify: ignored candidate predecessor",
			logger.FNode("current_predecessor", pred),
			logger.FNode("candidate", m),
		)
	}
}
