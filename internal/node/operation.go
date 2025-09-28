package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/ctxutil"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsValidID checks whether the given ID is valid
// in the identifier space of this node.
func (n *Node) IsValidID(bytes []byte) error {
	return n.rt.Space().IsValidID(bytes)
}

// Space returns the identifier space used by this node.
func (n *Node) Space() *domain.Space {
	return n.rt.Space()
}

// findNextHop scans a circular, ordered list of nodes and determines
// the index of the node whose identifier immediately precedes currentI.
//
// Nil entries are skipped: if either endpoint of an interval is nil,
// the function advances to the next non-nil entry before comparing.
// Each skipped nil entry triggers a warning log.
//
// Returns the index of the predecessor node, or -1 if no valid interval is found.
func (n *Node) findNextHop(list []*domain.Node, currentI domain.ID) int {
	if len(list) == 0 {
		return -1
	}

	for i := 0; i < len(list); i++ {
		curr := list[i]
		if curr == nil {
			n.lgr.Warn("findNextHop: nil node in list", logger.F("index", i))
			continue
		}

		// find next non-nil
		j := (i + 1) % len(list)
		for list[j] == nil && j != i {
			n.lgr.Warn("findNextHop: skipping nil node in list", logger.F("index", j))
			j = (j + 1) % len(list)
		}
		if j == i { // all nil except curr
			break
		}

		next := list[j]
		if currentI.Between(curr.ID, next.ID) {
			return i
		}
	}

	return -1
}

// FindSuccessorInit questa funzione è quella che viene chiamata dal server se riceve una richiesta di FindSuccessor in modalità INIT
// ovvero senza currentI e kshift
// in questo caso la funzione deve iniziare la ricerca del successore partendo dal nodo corrente
// e seguendo la logica del protocollo Koorde
// è la funzione che faccio partire in qualsisasi caso in cui voglio avviare una lookup da me stesso
func (n *Node) FindSuccessorInit(ctx context.Context, target domain.ID) (*domain.Node, error) {
	// check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	self := n.rt.Self()
	// check if the target is in (self, successor]
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Error("routing table not initialized: successor is nil")
		return nil, status.Error(codes.Internal, "node not initialized (routing table not initialized)")
	} else if target.Between(self.ID, succ.ID) {
		n.lgr.Info("EndLookup: target in (self, successor], returning successor",
			logger.F("target", target.ToHexString(true)), logger.FNode("successor", succ))
		return succ, nil
	}
	// start de Bruijn routing
	Bruijn := n.rt.DeBruijnList()
	if Bruijn != nil && len(Bruijn) > 0 {
		// calculate digit and kshift
		digit, kshift, err := n.rt.Space().NextDigitBaseK(target)
		if err != nil {
			n.lgr.Error("FindSuccessorInit: failed to compute next digit and kshift",
				logger.F("target", target), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to compute next digit and kshift")
		}
		// compute currentI = k * ID + digit
		currentI, err := n.rt.Space().MulKMod(self.ID)
		if err != nil {
			n.lgr.Error("FindSuccessorInit: failed to compute currentI during MulKMod",
				logger.F("target", target), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to compute currentI")
		}
		currentI, err = n.rt.Space().AddMod(currentI, n.rt.Space().FromUint64(digit))
		if err != nil {
			n.lgr.Error("FindSuccessorInit: failed to compute currentI during AddMod",
				logger.F("target", target), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to compute currentI")
		}
		// find the closest preceding node to currentI
		index := n.findNextHop(Bruijn, currentI)
		for i := index; i >= 0; i-- {
			d := Bruijn[i]
			if d == nil {
				continue
			}
			n.lgr.Info("FindSuccessorStep: forwarding to de Bruijn node",
				logger.F("target", target), logger.FNode("nextHop", d))
			var res *domain.Node
			var err error
			if d.ID.Equal(self.ID) {
				res, err = n.FindSuccessorStep(ctx, target, currentI, kshift)
			} else {
				cli, err := n.cp.GetFromPool(d.Addr)
				if err != nil {
					n.lgr.Warn("FindSuccessorInit: failed to get connection from pool",
						logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
					continue
				}
				res, err = client.FindSuccessorStep(ctx, cli, n.Space(), target, currentI, kshift)
			}
			if err == nil && res != nil {
				return res, nil
			} else {
				// se il contesto è già scaduto/cancellato = stop immediato
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
					n.lgr.Error("FindSuccessorInit: lookup interrotto per timeout/cancel",
						logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
					return nil, ctx.Err()
				}
				// altrimenti logghiamo il problema e proviamo il precedente
				n.lgr.Warn("FindSuccessorInit: de Bruijn nodo errore",
					logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
			}
		}
	}
	// if all the nodes in the de Bruijn list not response, fallback to successor
	n.lgr.Warn("FindSuccessorInit: no de Bruijn responded or present, falling back to successor",
		logger.F("target", target), logger.FNode("successor", succ))
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		n.lgr.Error("FindSuccessorInit: failed to get connection from pool for successor",
			logger.F("addr", succ.Addr), logger.F("err", err))
		return nil, status.Error(codes.Internal, "failed to get connection to successor")
	}
	return client.FindSuccessorStart(ctx, cli, n.Space(), target)
}

// FindSuccessorStep questa funzione è quella che viene chiamata dal server se riceve una richiesta di FindSuccessor in modalità STEP
// ovvero con currentI e kshift
// in questo caso la funzione deve continuare la ricerca del successore partendo dal nodo corrente
// e seguendo la logica del protocollo Koorde
func (n *Node) FindSuccessorStep(ctx context.Context, target, currentI, kshift domain.ID) (*domain.Node, error) {
	// check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	self := n.rt.Self()
	// check if the target is in (self, successor]
	succ := n.rt.FirstSuccessor()
	if succ == nil {
		n.lgr.Error("routing table not initialized: successor is nil")
		return nil, status.Error(codes.Internal, "node not initialized (routing table not initialized)")
	} else if target.Between(self.ID, succ.ID) {
		n.lgr.Info("EndLookup: target in (self, successor], returning successor",
			logger.F("target", target), logger.FNode("successor", succ))
		return succ, nil
	}
	// start de Bruijn routing
	// check if currentI is in (self, successor]
	if currentI.Between(self.ID, succ.ID) {
		// extract the de Bruijn link
		Bruijn := n.rt.DeBruijnList()
		if Bruijn != nil && len(Bruijn) > 0 {
			nextDigit, nextKshift, err := n.rt.Space().NextDigitBaseK(kshift)
			if err != nil {
				n.lgr.Error("FindSuccessorStep: failed to compute next digit and kshift",
					logger.F("target", target), logger.F("err", err))
				return nil, status.Error(codes.Internal, "failed to compute next digit and kshift")
			}
			nextI, err := n.rt.Space().MulKMod(currentI)
			if err != nil {
				n.lgr.Error("FindSuccessorStep: failed to compute nextI during MulKMod",
					logger.F("target", target), logger.F("err", err))
				return nil, status.Error(codes.Internal, "failed to compute nextI")
			}
			nextI, err = n.rt.Space().AddMod(nextI, n.rt.Space().FromUint64(nextDigit))
			if err != nil {
				n.lgr.Error("FindSuccessorStep: failed to compute nextI during AddMod",
					logger.F("target", target), logger.F("err", err))
				return nil, status.Error(codes.Internal, "failed to compute nextI")
			}
			// find the closest preceding node to currentI
			index := n.findNextHop(Bruijn, nextI)
			for i := index; i >= 0; i-- {
				d := Bruijn[i]
				if d == nil {
					continue
				}
				n.lgr.Info("FindSuccessorStep: forwarding to de Bruijn node",
					logger.F("target", target), logger.FNode("nextHop", d))
				var res *domain.Node
				var err error
				if d.ID.Equal(self.ID) {
					res, err = n.FindSuccessorStep(ctx, target, nextI, nextKshift)
				} else {
					cli, err := n.cp.GetFromPool(d.Addr)
					if err != nil {
						n.lgr.Warn("FindSuccessorStep: failed to get connection from pool",
							logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
						continue
					}
					res, err = client.FindSuccessorStep(ctx, cli, n.Space(), target, nextI, nextKshift)
				}
				if err == nil && res != nil {
					return res, nil
				} else {
					// se il contesto è già scaduto/cancellato = stop immediato
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
						n.lgr.Error("FindSuccessorStep: lookup interrotto per timeout/cancel",
							logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
						return nil, ctx.Err()
					}
					// altrimenti logghiamo il problema e proviamo il precedente
					n.lgr.Warn("FindSuccessorStep: de Bruijn nodo errore",
						logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
				}
			}
		}
		// if all the nodes in the de Bruijn list not response, fallback to successor
		n.lgr.Warn("FindSuccessorStep: no de Bruijn responded or present, falling back to successor",
			logger.F("target", target), logger.FNode("nextHop", succ))
		cli, err := n.cp.GetFromPool(succ.Addr)
		if err != nil {
			n.lgr.Error("FindSuccessorStep: failed to get connection from pool for successor",
				logger.F("addr", succ.Addr), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to get connection to successor")
		}
		return client.FindSuccessorStart(ctx, cli, n.Space(), target)
	}
	// next hop is successor
	n.lgr.Info("FindSuccessorStep: forwarding to successor",
		logger.F("target", target), logger.FNode("nextHop", succ))
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		n.lgr.Error("FindSuccessorStep: failed to get connection from pool for successor",
			logger.F("addr", succ.Addr), logger.F("err", err))
		return nil, status.Error(codes.Internal, "failed to get connection to successor")
	}
	return client.FindSuccessorStep(ctx, cli, n.Space(), target, currentI, kshift)
}

// Self returns the node information of this node.
func (n *Node) Self() *domain.Node {
	self := n.rt.Self()
	n.lgr.Debug("Self: returning self node", logger.FNode("self", self))
	return self
}

// Predecessor returns the predecessor of this node as currently
// stored in the routing table.
func (n *Node) Predecessor() *domain.Node {
	pred := n.rt.GetPredecessor()
	n.lgr.Debug("Predecessor: returning current predecessor",
		logger.FNode("predecessor", pred))
	return pred
}

// SuccessorList returns the current successor list of this node from the routing table.
func (n *Node) SuccessorList() []*domain.Node {
	list := n.rt.SuccessorList()
	n.lgr.Debug("SuccessorList: returning current list",
		logger.F("count", len(list)))
	return list
}

// DeBruijnList returns the current de Bruijn list of this node from the routing table.
func (n *Node) DeBruijnList() []*domain.Node {
	list := n.rt.DeBruijnList()
	n.lgr.Debug("DeBruijnList: returning current list",
		logger.F("count", len(list)))
	return list
}

// Notify informs this node about a potential predecessor.
// If the notifying node p lies between the current predecessor
// and self, the predecessor is updated.
func (n *Node) Notify(p *domain.Node) {
	// check if the notifier is nil or self
	if p == nil || p.ID.Equal(n.rt.Self().ID) {
		n.lgr.Debug("Notify: notify called with nil or self node, ignored", logger.FNode("node", p))
		return
	}
	// get current predecessor
	pred := n.rt.GetPredecessor()
	// if no predecessor or p is between pred and self (or pred == self), update
	if pred == nil || p.ID.Between(pred.ID, n.rt.Self().ID) {
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
		// send to predecessor the resource for which it is now responsible
		resources := n.s.Between(p.ID, n.rt.Self().ID)
		ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
		defer cancel()
		cli, err := n.cp.GetFromPool(p.Addr)
		if err != nil {
			n.lgr.Warn("Notify: failed to get connection to new predecessor from pool", logger.F("node", p), logger.F("err", err))
			return
		}
		err = client.StoreRemote(ctx, cli, resources)
		if err != nil {
			n.lgr.Warn("Notify: failed to transfer resources to new predecessor", logger.F("node", p), logger.F("err", err), logger.F("resourceCount", len(resources)))
		}
		// log update
		n.lgr.Info("Notify: predecessor updated", logger.FNode("newPredecessor", p), logger.FNode("oldPredecessor", pred))
	}
}

// Put stores a resource in the DHT on behalf of an external client.
// The node computes the ID of the key, finds the successor responsible for it,
// and either stores the resource locally (if it is the successor) or forwards
// the request to the successor node.
//
// Context is propagated so that timeouts and cancellations from the client
// apply also to the routing and storage steps.
func (n *Node) Put(ctx context.Context, res *domain.Resource) error {
	// Check if the context has already been canceled or expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return err
	}
	// Find the successor node responsible for this ID
	succ, err := n.FindSuccessorInit(ctx, res.Key)
	if err != nil {
		return fmt.Errorf("put: failed to find successor for key %s: %w", res.RawKey, err)
	}
	if succ == nil {
		return fmt.Errorf("put: no successor found for key %s", res.RawKey)
	}
	// If this node is the successor, store locally
	if succ.ID.Equal(n.rt.Self().ID) {
		return n.StoreLocal(ctx, *res)
	}
	sres := []domain.Resource{*res} // wrap in slice for StoreRemote
	// Otherwise, forward the resource to the successor
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		return fmt.Errorf("put: failed to get connection to successor %s: %w", succ.Addr, err)
	}
	if err := client.StoreRemote(ctx, cli, sres); err != nil {
		return fmt.Errorf("put: failed to store resource at successor %s: %w", succ.Addr, err)
	}
	// Log success
	n.lgr.Info("Put: resource stored at successor", logger.F("key", res.RawKey), logger.FNode("successor", succ))
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
func (n *Node) Get(ctx context.Context, id domain.ID) (*domain.Resource, error) {
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	succ, err := n.FindSuccessorInit(ctx, id) // is used the context from client
	if err != nil {
		return nil, fmt.Errorf("get: failed to find successor for key %s: %w", id.ToHexString(true), err)
	}
	if succ == nil {
		return nil, fmt.Errorf("get: no successor found for key %s", id.ToHexString(true))
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
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		return nil, fmt.Errorf("get: failed to get connection to successor %s: %w", succ.Addr, err)
	}
	res, err := client.RetrieveRemote(ctx, cli, n.Space(), id)
	if err != nil {
		return nil, fmt.Errorf("get: failed to retrieve resource from successor %s: %w", succ.Addr, err)
	}
	return res, nil
}

// Delete removes a resource from the DHT on behalf of an external client.
// The node computes the ID of the key, finds the successor responsible for it,
// and either deletes the resource locally or forwards the request to the
// successor node.
//
// Returns NotFound if the resource does not exist.
func (n *Node) Delete(ctx context.Context, id domain.ID) error {
	if err := ctxutil.CheckContext(ctx); err != nil {
		return err
	}
	succ, err := n.FindSuccessorInit(ctx, id)
	if err != nil {
		return fmt.Errorf("delete: failed to find successor for key %s: %w", id.ToHexString(true), err)
	}
	if succ == nil {
		return fmt.Errorf("delete: no successor found for key %s", id.ToHexString(true))
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
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		return fmt.Errorf("delete: failed to get connection to successor %s: %w", succ.Addr, err)
	}
	if err := client.RemoveRemote(ctx, cli, id); err != nil {
		return fmt.Errorf("delete: failed to remove resource at successor %s: %w", succ.Addr, err)
	}
	return nil
}

// StoreLocal memorizza la risorsa nel nodo locale utilizzando lo storage interno. (chiamata da operazioni node -> node)
func (n *Node) StoreLocal(ctx context.Context, resource domain.Resource) error {
	pred := n.rt.GetPredecessor()
	// Se non ha predecessore -> considerati responsabile (fase bootstrap)
	if pred == nil || resource.Key.Between(pred.ID, n.rt.Self().ID) {
		n.s.Put(resource)
		return nil
	}
	// Non sono responsabile → tenta forwarding
	if err := n.Put(ctx, &resource); err != nil {
		// qui ritorniamo errore reale, utile per capire se è problema di routing
		return fmt.Errorf("forwarding store to successor failed: %w", err)
	}
	return nil
}

// RetrieveLocal ottiene la risorsa con la chiave specificata dal nodo locale utilizzando lo storage interno. (chiamata da operazioni node -> node)
func (n *Node) RetrieveLocal(id domain.ID) (domain.Resource, error) {
	return n.s.Get(id)
}

// RemoveLocal rimuove la risorsa con la chiave specificata dal nodo locale utilizzando lo storage interno. (chiamata da operazioni node -> node)
func (n *Node) RemoveLocal(id domain.ID) error {
	return n.s.Delete(id)
}

// GetAllResourceStored returns the internal storage used by this node.
func (n *Node) GetAllResourceStored() []domain.Resource {
	return n.s.All()
}

func (n *Node) LookUp(ctx context.Context, id domain.ID) (*domain.Node, error) {
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	succ, err := n.FindSuccessorInit(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get: failed to find successor for key %s: %w", id.ToHexString(true), err)
	}
	if succ == nil {
		return nil, fmt.Errorf("get: no successor found for key %s", id.ToHexString(true))
	}
	return succ, nil
}

func (n *Node) HandleLeave(leaveNode *domain.Node) error {
	if leaveNode == nil || !leaveNode.ID.Equal(n.rt.GetPredecessor().ID) {
		n.lgr.Warn("HandleLeave: ignoring leave for nil or non-predecessor node")
		return fmt.Errorf("handle leave: ignoring leave for nil or non-predecessor node")
	}
	// rimuovi il nodo da predecessore
	n.rt.SetPredecessor(nil)
	// rilascia la connessione dal pool
	if err := n.cp.Release(leaveNode.Addr); err != nil {
		n.lgr.Warn("HandleLeave: failed to release leaving node from pool",
			logger.F("node", leaveNode), logger.F("err", err))
	}
	n.lgr.Info("HandleLeave: node removed from routing table and connection pool",
		logger.FNode("leavingNode", leaveNode))
	return nil
}
