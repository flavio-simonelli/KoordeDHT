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

func (n *Node) Put(r domain.Resource) error {
	err := n.s.Put(r)
	if err != nil {
		return err
	}
	n.lgr.Info("Put: resource stored", logger.FResource("resource", r))
	return nil
}

func (n *Node) Get(key domain.ID) (domain.Resource, error) {
	res, err := n.s.Get(key)
	if err != nil {
		return domain.Resource{}, err
	}
	// TODO: qui dobbiamo inviare la richiesta di get al nodo che ha la risorsa se io non ce l'ho localmente
	n.lgr.Info("Get: resource retrieved", logger.FResource("resource", res))
	return res, nil
}

func (n *Node) Delete(key domain.ID) error {
	err := n.s.Delete(key)
	// TODO: qui dobbiamo inviare la richiesta di delete al nodo che ha la risorsa se io non sono il responsabile
	if err != nil {
		return err
	}
	n.lgr.Info("Delete: resource deleted", logger.F("key", key.ToHexString()))
	return nil
}
