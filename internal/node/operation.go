package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	var startIdx = -1
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
		n.lgr.Info("Notify: updating predecessor", logger.FNode("new", *p))
		// addRef new predecessor
		if err := n.cp.AddRef(p.Addr); err != nil {
			n.lgr.Warn("Notify: failed to add new predecessor to pool",
				logger.F("node", p), logger.F("err", err))
		}
		// set new predecessor
		n.rt.SetPredecessor(p)
		// release old predecessor (if p not nil)
		if pred != nil {
			if err := n.cp.Release(pred.Addr); err != nil {
				n.lgr.Warn("Notify: failed to release old predecessor",
					logger.F("node", pred), logger.F("err", err))
			}
		}
	}
}

func (n *Node) CheckIdValidity(id domain.ID) error {
	return n.rt.Space().IsValidID(id)
}

// Put stores a resource in the DHT on behalf of an external client.
// The node computes the ID of the key, finds the successor responsible for it,
// and either stores the resource locally (if it is the successor) or forwards
// the request to the successor node.
//
// Context is propagated so that timeouts and cancellations from the client
// apply also to the routing and storage steps.
func (n *Node) Put(ctx context.Context, key string, value string) error {
	// Check if the context has already been canceled or expired
	if err := checkContext(ctx); err != nil {
		return err
	}
	// Translate the client key into a DHT identifier
	id := n.rt.Space().NewIdFromString(key)
	// Find the successor node responsible for this ID
	succ, err := n.FindSuccessorInit(ctx, id)
	if err != nil {
		return fmt.Errorf("Put: failed to find successor for key %s: %w", key, err)
	}
	if succ == nil {
		return fmt.Errorf("Put: no successor found for key %s", key)
	}
	// Build the resource object
	res := domain.Resource{
		Key:   id,
		Value: value,
	}
	// If this node is the successor, store locally
	if succ.ID.Equal(n.rt.Self().ID) {
		return n.StoreLocal(res)
	}
	// Otherwise, forward the resource to the successor
	if err := n.cp.StoreRemoteWithContext(ctx, res, succ.Addr); err != nil {
		return fmt.Errorf("Put: failed to store resource at successor %s: %w", succ.Addr, err)
	}
	// Log success
	n.lgr.Info("Put: resource stored at successor", logger.F("key", key), logger.FNode("successor", *succ))
	return nil
}

// Get retrieves a resource from the DHT on behalf of an external client.
// The node computes the ID of the key, finds the successor responsible for it,
// and either fetches the resource locally or forwards the request to the
// successor node.
//
// Returns:
//   - *domain.Resource if found
//   - status.Error(codes.NotFound, ...) if the resource does not exist
//   - error in case of routing or RPC issues
func (n *Node) Get(ctx context.Context, key string) (*domain.Resource, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	id := n.rt.Space().NewIdFromString(key)
	succ, err := n.FindSuccessorInit(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Get: failed to find successor for key %s: %w", key, err)
	}
	if succ == nil {
		return nil, fmt.Errorf("Get: no successor found for key %s", key)
	}
	// If this node is the successor, retrieve locally
	if succ.ID.Equal(n.rt.Self().ID) {
		res, err := n.RetrieveLocal(id)
		if err != nil {
			if errors.Is(err, domain.ErrResourceNotFound) {
				return nil, status.Error(codes.NotFound, "key not found")
			}
			return nil, err
		}
		return &res, nil
	}
	// Otherwise, forward the request to the successor
	res, err := n.cp.RetrieveRemoteWithContext(ctx, id, succ.Addr)
	if err != nil {
		return nil, fmt.Errorf("Get: failed to retrieve resource from successor %s: %w", succ.Addr, err)
	}
	return res, nil
}

// Delete removes a resource from the DHT on behalf of an external client.
// The node computes the ID of the key, finds the successor responsible for it,
// and either deletes the resource locally or forwards the request to the
// successor node.
//
// Returns NotFound if the resource does not exist.
func (n *Node) Delete(ctx context.Context, key string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	id := n.rt.Space().NewIdFromString(key)
	succ, err := n.FindSuccessorInit(ctx, id)
	if err != nil {
		return fmt.Errorf("Delete: failed to find successor for key %s: %w", key, err)
	}
	if succ == nil {
		return fmt.Errorf("Delete: no successor found for key %s", key)
	}
	// If this node is the successor, delete locally
	if succ.ID.Equal(n.rt.Self().ID) {
		if err := n.RemoveLocal(id); err != nil {
			if errors.Is(err, domain.ErrResourceNotFound) {
				return status.Error(codes.NotFound, "key not found")
			}
			return err
		}
		return nil
	}
	// Otherwise, forward the request to the successor
	if err := n.cp.RemoveRemoteWithContext(ctx, id, succ.Addr); err != nil {
		return fmt.Errorf("Delete: failed to remove resource at successor %s: %w", succ.Addr, err)
	}
	return nil
}

// StoreLocal memorizza la risorsa nel nodo locale utilizzando lo storage interno. (chiamata da operazioni node -> node)
func (n *Node) StoreLocal(resource domain.Resource) error {
	return n.s.Put(resource)
}

// RetrieveLocal ottiene la risorsa con la chiave specificata dal nodo locale utilizzando lo storage interno. (chiamata da operazioni node -> node)
func (n *Node) RetrieveLocal(id domain.ID) (domain.Resource, error) {
	return n.s.Get(id)
}

// RemoveLocal rimuove la risorsa con la chiave specificata dal nodo locale utilizzando lo storage interno. (chiamata da operazioni node -> node)
func (n *Node) RemoveLocal(id domain.ID) error {
	return n.s.Delete(id)
}

// checkContext checks whether the provided context has been canceled
// or has exceeded its deadline. If so, it returns the corresponding
// gRPC status error. Otherwise, it returns nil.
func checkContext(ctx context.Context) error {
	switch err := ctx.Err(); {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request was canceled by client")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return nil
	}
}
