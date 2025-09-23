package node

import (
	"KoordeDHT/internal/logger"
	"context"
	"fmt"
	"time"
)

// StartStabilizer avvia una goroutine che esegue periodicamente stabilize.
func (n *Node) StartStabilizer(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				n.lgr.Info("stabilizer stopped")
				return
			case <-ticker.C:
				n.stabilizeSuccessor()
				n.stabilizeDeBruijn()
			}
		}
	}()
}

func (n *Node) stabilizeSuccessor() {
	// Vedo chi è il mio successore
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Warn("stabilize: successor is nil, routing table not initialized")
		return
	}
	// Caso: il successore sono io stesso
	if succ.ID.Equal(n.rt.Self().ID) {
		pred := n.rt.GetPredecessor()
		if pred != nil && !pred.ID.Equal(n.rt.Self().ID) {
			// aggiorno il mio successore con il mio predecessore
			n.rt.SetSuccessor(0, pred)
			succ = pred
			// notifico il nuovo successore
			if err := n.cp.Notify(n.rt.Self(), succ.Addr); err != nil {
				n.lgr.Warn("failed to notify new successor", logger.F("error", err.Error()))
			}
		}
	} else {
		// Caso: il successore non sono io
		pred, err := n.cp.GetPredecessor(succ.Addr)
		if err != nil {
			n.lgr.Warn("failed to get predecessor from successor", logger.FNode("successor", *succ), logger.F("error", err.Error()))
			return
		}
		// Se pred.ID ∈ (n.ID, n.Successor.ID) → aggiorno successore
		if pred.ID.Between(n.rt.Self().ID, succ.ID) {
			n.rt.SetSuccessor(0, &pred)
			n.lgr.Info("updated successor to closer predecessor", logger.FNode("new_successor", pred))
		} else {
			// Notifico il successore che io sono il suo vero predecessore
			if err := n.cp.Notify(n.rt.Self(), succ.Addr); err != nil {
				n.lgr.Warn("failed to notify successor", logger.F("error", err.Error()))
			}
		}
	}
	// In qualunque caso → aggiorno la successor list
	if err := n.updateSuccessorList(); err != nil {
		n.lgr.Warn("failed to update successor list", logger.F("error", err.Error()))
	}
}

// updateSuccessorList aggiorna la lista dei successori contattando il successore corrente
func (n *Node) updateSuccessorList() error {
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		return fmt.Errorf("successor is nil")
	}
	list, err := n.cp.GetSuccessorList(succ.Addr)
	if err != nil {
		return err
	}
	// aggiorniamo la nostra successor list
	for i := 1; i < len(n.rt.SuccessorList()); i++ {
		if i-1 < len(list) {
			n.rt.SetSuccessor(i, &list[i-1])
		} else {
			// se la lista del successore è più corta della nostra, mettiamo nil
			n.rt.SetSuccessor(i, nil)
		}
	}
	return nil
}

func (n *Node) stabilizeDeBruijn() {
	//TODO: implementare
	// qui bisogna contattare il primo nodo della de Bruijn e chiedendo il predecessore id perd(km)
	// se non risponde devo contattare il mio successore per trovarlo
	// se risponde ed è lui gli chiedo la sua successor list per aggiornare i miei link
	// se risponde e non è lui chiedo al vero pred la sua successor list e aggiorno tutto
}
