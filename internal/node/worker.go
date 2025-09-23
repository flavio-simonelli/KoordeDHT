package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"time"
)

func (n *Node) StartBackgroundTasks() {
	go n.periodic("fixSuccessor", 2*time.Second, n.FixSuccessorList)
	//go n.periodic("fixPredecessor", 3*time.Second, n.FixPredecessor)
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
	required := degreeGraph
	successors := make([]domain.Node, 0, required)
	current := anchorPred

	for len(successors) < required {
		var succList []domain.Node
		var err error

		if current.ID.Equal(n.rt.Self().ID) {
			// se current sono io, uso direttamente la mia successor list
			succList = n.rt.SuccessorList()
		} else {
			// contatta current per ottenere la sua successor list
			succList, err = n.cp.GetSuccessorList(current.Addr)
			if err != nil {
				n.lgr.Error("errore nel contattare il nodo per la successor list",
					logger.FNode("node", current), logger.F("error", err))
				break
			}
		}

		// aggiungi successori alla lista finché non raggiungi il numero richiesto
		for _, s := range succList {
			if len(successors) >= required {
				break
			}
			successors = append(successors, s) //TODO: controlla duplicati?
		}

		// aggiorna current con l'ultimo nodo aggiunto
		if len(successors) > 0 {
			current = successors[len(successors)-1]
		} else {
			// non ci sono successori validi → esci
			n.lgr.Warn("FixDebruijnLinks: non ci sono successori disponibili per continuare")
			break
		}
	}

	// --- Aggiorna i link De Bruijn ---
	for i, node := range successors {
		n.rt.FixDeBruijn(i+1, node) // i+1 perché indice 0 è anchor
	}

}

func (n *Node) FixSuccessorList() {
	self := n.rt.Self()
	contacted := false // flag: se siamo riusciti a contattare almeno un successore valido

	for i := 0; i < len(n.rt.SuccessorList()); i++ {
		succ, err := n.rt.Successor(i)
		if err != nil {
			n.lgr.Warn("FixSuccessor: impossibile ottenere successore", logger.F("error", err))
			return
		}
		// se il successore è me stesso, senza grpc vedo chi è il mio predecessore
		if succ.ID.Equal(self.ID) {
			pred := n.rt.Predecessor()
			if !pred.ID.Equal(self.ID) {
				n.rt.SetSuccessor(0, pred)
				contacted = true
			} else {
				// se anche il predecessore è me stesso, non ho altri nodi nella rete
				return
			}
		} else {
			// chiedo al mio successore chi è il suo predecessore
			pred, err := n.cp.GetPredecessor(succ.Addr)
			if err != nil {
				n.lgr.Warn("FixSuccessor: successor not responding, fallback",
					logger.FNode("successor", succ), logger.F("error", err.Error()))
				// passiamo al prossimo successore
				continue
			}
			contacted = true
			// se il predecessore sono io allora non faccio nulla
			if pred.ID.Equal(self.ID) {
				break
			} else if pred.ID.InOO(self.ID, succ.ID) {
				// se il predecessore del successore è tra me e il successore, allora aggiorno
				n.rt.SetSuccessor(0, pred)
			} else {
				// altrimenti è diverso da me ma è più piccolo di me quindi notifico il mio successore
				err := n.cp.Notify(self, succ.Addr)
				if err != nil {
					return
				}
				break
			}
		}
	}
	// se non sono riuscito a contattare nessun successore valido, rieffettuo la join
	if !contacted {
		n.lgr.Warn("FixSuccessor: non sono riuscito a contattare nessun successore valido, rieffettuo la join")
		// resetto il predecessore a me stesso
		n.rt.SetPredecessor(n.rt.Self())
		// resetto la successor list
		for i := 0; i < len(n.rt.SuccessorList()); i++ {
			n.rt.SetSuccessor(i, n.rt.Self())
		}
		// provo a fare la join
		// qui dovrei avere l'indirizzo di un nodo di bootstrap salvato da qualche parte
		// per ora lascio un log
		n.lgr.Warn("FixSuccessor: join not implemented, please re-join the network manually")
		return
	}
	// se sono qui, ho un successore valido in posizione 0
	// ora aggiorno la successor list
	successor, err := n.rt.Successor(0)
	if err != nil {
		return
	}
	// contatto il successore per ottenere la sua successor list
	succList, err := n.cp.GetSuccessorList(successor.Addr)
	if err != nil {
		n.lgr.Error("FixSuccessor: get successor list failed",
			logger.FNode("successor", successor), logger.F("error", err.Error()))
		return
	}
	// aggiorna la mia successor list con i primi elementi della successor list del successore
	for i := 1; i < len(n.rt.SuccessorList()) && i < len(succList); i++ {
		n.rt.SetSuccessor(i, succList[i-1])
	}
}
