package node

import (
	"KoordeDHT/internal/logger"
	"time"
)

func (n *Node) StartBackgroundTasks() {
	go n.periodic("fixSuccessor", 2*time.Second, n.FixSuccessor)
	go n.periodic("fixPredecessor", 3*time.Second, n.FixPredecessor)
	go n.periodic("fixDebruijn", 5*time.Second, n.FixDebruijnLinks)
}

func (n *Node) periodic(name string, interval time.Duration, fn func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			n.lgr.Debug("running background task", logger.F("task", name))
			fn()
			/*
				case <-n.ctx.Done(): // se usi un context per gestire la vita del nodo
					n.lgr.Info("stopping background task", logger.F("task", name))
					return
			*/
		}
	}
}

func (n *Node) FixDebruijnLinks() {
	degreeGraph := n.rt.Degree()
	idBits := n.rt.IDBits()
	myId := n.rt.Self().ID
	for i := 0; i < degreeGraph; i++ {
		// calcola l'id De Bruijn all'indice i
		dbId, err := myId.DeBruijnNext(degreeGraph, i, idBits)
		if err != nil {
			n.lgr.Error("errore nel calcolo dell'id De Bruijn", logger.F("index", i), logger.F("error", err))
		}
		// trova il successore dell'id De Bruijn
		succ, err := n.FindSuccessor(dbId)
		if err != nil {
			n.lgr.Error("errore nel trovare il successore dell'id De Bruijn", logger.F("debruijn_id", dbId.ToHexString()), logger.F("error", err))
			continue
		}
		// aggiorna il link De Bruijn all'indice i
		n.rt.FixDeBruijn(i, succ)
		n.lgr.Info("aggiornato link De Bruijn",
			logger.F("index", i),
			logger.F("debruijn_id", dbId.ToHexString()),
			logger.FNode("successor", succ),
		)
	}
}

func (n *Node) FixSuccessor() {
	// Chiedi al mio successore il suo predecessore
	succ := n.rt.Successor()
	// se  successore coincide con me, non c'è nulla da fare
	if succ.ID.Equal(n.rt.Self().ID) {
		return
	}
	predOfSucc, err := n.cp.GetPredecessor(succ.Addr)
	if err != nil {
		n.lgr.Warn("FixSuccessor: successor not responding, fallback",
			logger.FNode("successor", succ), logger.F("error", err.Error()))
		// TODO: in caso di failure multipli, potrei iterare su backup/finger table (qui devo contattare i successore del successore)
		return
	}
	self := n.rt.Self()
	if predOfSucc.ID.InOO(self.ID, succ.ID) {
		n.rt.SetSuccessor(predOfSucc)
		succ = predOfSucc
	}
	if err := n.cp.Notify(self, succ.Addr); err != nil {
		n.lgr.Error("FixSuccessor: notify failed",
			logger.FNode("successor", succ), logger.F("error", err.Error()))
	}
}

func (n *Node) FixPredecessor() {
	pred := n.rt.Predecessor()
	// se predecessore coincide con me, non c'è nulla da fare
	if pred.ID.Equal(n.rt.Self().ID) {
		return
	}
	// provo a pingarlo
	err := n.cp.Ping(pred.Addr)
	if err != nil {
		n.lgr.Warn("FixPredecessor: predecessor not responding, resetting",
			logger.FNode("predecessor", pred), logger.F("error", err.Error()))
		n.rt.SetPredecessor(n.rt.Self()) //TODO: dovrei fare find node per trovare il predecessore
	}
}
