package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"errors"
	"fmt"
)

// FindSuccessorInit questa funzione è quella che viene chiamata dal server se riceve una richiesta di FindSuccessor in modalità INIT
// ovvero senza currentI e kshift
// in questo caso la funzione deve iniziare la ricerca del successore partendo dal nodo corrente
// e seguendo la logica del protocollo Koorde
func (n *Node) FindSuccessorInit(ctx context.Context, target domain.ID) (*domain.Node, error) {
	// check if the target id is between the current node and its successor
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Warn("FindSuccessorInit: successor non inizializzato")
		return nil, errors.New("successor not initialized")
	}
	if target.Between(self.ID, succ.ID) {
		return succ, nil
	}
	// otherwise first hop of de Bruijn graph routing
	debruijn := n.rt.DeBruijnList()
	if debruijn == nil || len(debruijn) == 0 {
		n.lgr.Warn("FindSuccessorInit: de Bruijn list non inizializzata")
		// fallback to successor
		return n.cp.FindSuccessorStartWithContext(ctx, target, succ.Addr)
	}
	digit, kshift := n.rt.Space().NextDigitBaseK(target)
	currentI := n.rt.Space().MulKMod(self.ID)
	currentI = n.rt.Space().AddMod(currentI, n.rt.Space().FromUint64(digit))
	// find the closest preceding node to currentI
	var startIdx int = -1
	for i := 0; i < len(debruijn)-2; i++ {
		cand, nxt := debruijn[i], debruijn[i+1]
		if cand == nil || nxt == nil {
			continue
		}
		if currentI.Between(cand.ID, nxt.ID) {
			startIdx = i
			break
		}
	}
	// se non trovato, prendi l’ultimo nodo valido
	if startIdx == -1 {
		for i := len(debruijn) - 1; i >= 0; i-- {
			if debruijn[i] != nil {
				startIdx = i
				break
			}
		}
	}
	// se abbiamo trovato qualcosa, proviamo a partire da lì e usiamo come fallback il precedente
	for i := startIdx; i >= 0; i-- {
		d := debruijn[i]
		if d == nil {
			continue
		}
		res, err := n.cp.FindSuccessorStepWithContext(ctx, target, currentI, kshift, d.Addr)
		if err == nil && res != nil {
			return res, nil
		}
		// se il contesto è già scaduto/cancellato = stop immediato
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			n.lgr.Error("FindSuccessorInit: lookup interrotto per timeout/cancel",
				logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
			return nil, ctx.Err()
		}
		// altrimenti logghiamo il problema e proviamo il precedente
		n.lgr.Warn("FindSuccessorInit: de Bruijn nodo non risponde",
			logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
	}
	// if all the nodes in the de Bruijn list not response, fallback to successor
	res, err := n.cp.FindSuccessorStartWithContext(ctx, target, succ.Addr)
	if err == nil && res != nil {
		return res, nil
	}
	n.lgr.Error("FindSuccessorInit: nessun nodo risponde, errore finale",
		logger.F("target", target), logger.F("succ", succ.Addr))
	return nil, fmt.Errorf("no de Bruijn or successor responded for target %s", target)
}

// FindSuccessorStep questa funzione è quella che viene chiamata dal server se riceve una richiesta di FindSuccessor in modalità STEP
// ovvero con currentI e kshift
// in questo caso la funzione deve continuare la ricerca del successore partendo dal nodo corrente
// e seguendo la logica del protocollo Koorde
func (n *Node) FindSuccessorStep(ctx context.Context, target, currentI, kshift domain.ID) (*domain.Node, error) {
	// check if the target id is between the current node and its successor
	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Warn("FindSuccessorInit: successor non inizializzato")
		return nil, errors.New("successor not initialized")
	}
	if target.Between(self.ID, succ.ID) {
		return succ, nil
	}
	// otherwise continue hop of de Bruijn graph routing
	if currentI.Between(self.ID, succ.ID) {
		// extract the de Bruijn link
		debruijn := n.rt.DeBruijnList()
		if debruijn == nil || len(debruijn) == 0 {
			n.lgr.Warn("FindSuccessorInit: de Bruijn list non inizializzata")
			// fallback to successor
			return n.cp.FindSuccessorStartWithContext(ctx, target, succ.Addr)
		}
		nextdigit, nextkshift := n.rt.Space().NextDigitBaseK(kshift)
		nextI := n.rt.Space().MulKMod(currentI)
		nextI = n.rt.Space().AddMod(nextI, n.rt.Space().FromUint64(nextdigit))
		// find the closest preceding node to currentI
		var startIdx = -1
		for i := 0; i < len(debruijn)-2; i++ {
			cand, nxt := debruijn[i], debruijn[i+1]
			if cand == nil || nxt == nil {
				continue
			}
			if nextI.Between(cand.ID, nxt.ID) {
				startIdx = i
				break
			}
		}
		// se non trovato, prendi l’ultimo nodo valido
		if startIdx == -1 {
			for i := len(debruijn) - 1; i >= 0; i-- {
				if debruijn[i] != nil {
					startIdx = i
					break
				}
			}
		}
		// se abbiamo trovato qualcosa, proviamo a partire da lì e usiamo come fallback il precedente
		for i := startIdx; i >= 0; i-- {
			d := debruijn[i]
			if d == nil {
				continue
			}
			res, err := n.cp.FindSuccessorStepWithContext(ctx, target, nextI, nextkshift, d.Addr)
			if err == nil && res != nil {
				return res, nil
			}
			// se il contesto è già scaduto/cancellato = stop immediato
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
				n.lgr.Error("FindSuccessorStep: lookup interrotto per timeout/cancel",
					logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
				return nil, ctx.Err()
			}
			// altrimenti logghiamo il problema e proviamo il precedente
			n.lgr.Warn("FindSuccessorStep: de Bruijn nodo non risponde",
				logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
		}
		// if all the nodes in the de Bruijn list not response, fallback to successor
		return n.cp.FindSuccessorStartWithContext(ctx, target, succ.Addr)
	} else {
		// next hop is successor
		return n.cp.FindSuccessorStepWithContext(ctx, target, currentI, kshift, succ.Addr)
	}
}

func (n *Node) Predecessor() *domain.Node {
	return n.rt.GetPredecessor()
}

func (n *Node) SuccessorList() []*domain.Node {
	return n.rt.SuccessorList()
}

func (n *Node) Notify(p *domain.Node) {
	if p == nil || p.ID.Equal(n.rt.Self().ID) {
		return
	}
	pred := n.rt.GetPredecessor()
	if pred == nil || p.ID.Between(pred.ID, n.rt.Self().ID) {
		n.lgr.Info("Notify: updating predecessor",
			logger.F("old", pred), logger.F("new", p))

		if pred != nil {
			if err := n.cp.Release(pred.Addr); err != nil {
				n.lgr.Warn("Notify: failed to release old predecessor",
					logger.F("node", pred), logger.F("err", err))
			}
		}
		if err := n.cp.AddRef(p.Addr); err != nil {
			n.lgr.Warn("Notify: failed to add new predecessor to pool",
				logger.F("node", p), logger.F("err", err))
		}

		n.rt.SetPredecessor(p)
	}
}
