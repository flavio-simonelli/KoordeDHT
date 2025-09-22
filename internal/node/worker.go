package node

import (
	"KoordeDHT/internal/domain"
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
	myId := n.rt.Self().ID
	// calcola l'id anchor (De Bruijn all'indice 0)
	anchorId := myId.AdvanceDeBruijn(0, degreeGraph)
	// trova il predecessore dell'id anchor
	anchorPred, err := n.FindPredecessorInit(anchorId)
	if err != nil {
		n.lgr.Error("errore nel trovare il predecessore dell'id anchor", logger.F("anchor_id", anchorId.ToHexString()), logger.F("error", err))
		return
	}
	// aggiorna il link De Bruijn all'indice 0 (anchor)
	n.rt.FixDeBruijn(0, anchorPred)
	// per ogni indice i da 1 a degreeGraph metti il successore del nodo precedente
	current := anchorPred
	for i := 1; i < degreeGraph+1; i++ {
		// ccontatta il nodo i-1 per consocere il suo successore
		// se il nodo precedente coincide con me, metti il mio successore
		if current.ID.Equal(n.rt.Self().ID) {
			n.rt.FixDeBruijn(i, n.rt.Successor())
			current = n.rt.Successor()
			continue
		}
		// contatta il nodo current per conoscere il suo successore
		succ, err := n.cp.GetSuccessor(current.Addr)
		if err != nil {
			n.lgr.Error("errore nel contattare il nodo precedente per conoscere il suo successore", logger.FNode("node", current), logger.F("error", err))
			break
		}
		// aggiorna il link De Bruijn all'indice i
		n.rt.FixDeBruijn(i, succ)
		current = succ
	}
}

func (n *Node) FixSuccessor() {
	// Chiedi al mio successore il suo predecessore
	succ := n.rt.Successor()
	self := n.rt.Self()
	var predOfSucc domain.Node
	var err error
	// se  successore coincide con me, prendi il tuo predecessore
	if succ.ID.Equal(self.ID) {
		predOfSucc = n.rt.Predecessor()
		if !predOfSucc.ID.Equal(self.ID) {
			n.rt.SetSuccessor(predOfSucc)
			succ = predOfSucc
		}
	} else {
		predOfSucc, err = n.cp.GetPredecessor(succ.Addr)
		if err != nil {
			n.lgr.Warn("FixSuccessor: successor not responding, fallback",
				logger.FNode("successor", succ), logger.F("error", err.Error()))
			// TODO: in caso di failure multipli, potrei iterare su backup/finger table (qui devo contattare i successore del successore)
			return
		}
		if predOfSucc.ID.InOO(self.ID, succ.ID) {
			n.rt.SetSuccessor(predOfSucc)
			succ = predOfSucc
		}
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
