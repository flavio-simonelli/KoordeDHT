package node

import (
	"KoordeDHT/internal/client"
	"KoordeDHT/internal/ctxutil"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IsValidID checks whether the provided identifier is valid
// within the identifier space of this node.
//
// A valid identifier must:
//   - Match the expected byte length of the configured space.
//   - Respect any bit masking rules (e.g., unused high-order bits
//     must be zero if the space size is not a multiple of 8).
//
// It delegates the actual validation logic to the Space associated
// with this node's routing table.
//
// Returns:
//   - nil if the identifier is valid.
//   - An error if the identifier is malformed or out of bounds.
func (n *Node) IsValidID(id []byte) error {
	return n.rt.Space().IsValidID(id)
}

// Space returns the identifier space of the DHT (used for validation and creation of the ID).
//
// Returns:
//   - A pointer to the Space instance used by this node.
func (n *Node) Space() *domain.Space {
	return n.rt.Space()
}

// findNextHop scans a circular, ordered list of nodes and determines
// the index of the node whose identifier immediately precedes currentI.
//
// Behavior:
//   - The list is treated as circular: the last node connects back to the first.
//   - Nil entries are tolerated and skipped.
//   - If the list contains only one valid element (even if surrounded by nils),
//     that element is returned.
//   - For each pair of consecutive non-nil nodes (curr, next), the function
//     checks whether currentI lies strictly between their identifiers.
//
// Returns:
//   - The index i of the predecessor node (list[i]) if found.
//   - -1 if no valid interval is found (e.g., list empty, all nil).
func (n *Node) findNextHop(list []*domain.Node, currentI domain.ID) int {
	if len(list) == 0 {
		return -1
	}

	for i := 0; i < len(list); i++ {
		curr := list[i]
		if curr == nil {
			continue
		}
		// Find the next non-nil node in circular fashion
		j := (i + 1) % len(list)
		for list[j] == nil {
			n.lgr.Warn("findNextHop: skipping nil node in list",
				logger.F("index", j))
			j = (j + 1) % len(list)
		}
		next := list[j]
		if currentI.Between(curr.ID, next.ID) {
			return i
		}
	}

	return -1
}

// FindSuccessorInit starts a successor lookup from this node.
//
// This method is invoked when a lookup request arrives in INIT mode,
// i.e. without an initial imaginary node (currentI) or shifted target (kshift).
// In this case, the lookup begins at the local node and follows the Koorde
// routing logic.
//
// Behavior:
//   - If the target lies in (self, successor], the lookup ends immediately
//     and the successor is returned.
//   - Otherwise, the method computes the initial imaginary node currentI
//     and the shifted target kshift using BestImaginarySimple, and forwards
//     the request to FindSuccessorStep for continued routing.
//
// Errors:
//   - Returns an error if the routing table is not initialized (successor is nil).
//   - Returns an error if initial currentI and kshift cannot be computed.
func (n *Node) FindSuccessorInit(ctx context.Context, target domain.ID) (*domain.Node, error) {
	// Abort if context expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()

	// check if the target is in (self, successor]
	if succ == nil {
		n.lgr.Error("routing table not initialized: successor is nil")
		return nil, status.Error(codes.Internal, "node not initialized (routing table not initialized)")
	}
	if target.Between(self.ID, succ.ID) {
		n.lgr.Debug("EndLookup: target in (self, successor], returning successor",
			logger.F("target", target.ToHexString(true)), logger.FNode("successor", succ))
		return succ, nil
	}

	// Compute initial imaginary node and shifted target
	currentI, kshift, err := n.rt.Space().BestImaginarySimple(self.ID, succ.ID, target)
	if err != nil {
		n.lgr.Error("FindSuccessorInit: failed to compute initial currentI and kshift",
			logger.F("target", target.ToHexString(true)), logger.F("err", err))
		return nil, status.Error(codes.Internal, "failed to compute initial currentI and kshift")
	}

	// Continue the lookup in STEP mode
	return n.FindSuccessorStep(ctx, target, currentI, kshift)
}

// FindSuccessorStep continues a successor lookup from this node.
//
// This method is invoked when a lookup request arrives in STEP mode,
// i.e. with an imaginary node currentI and shifted target kshift already set.
// It follows the Koorde routing logic:
//
// Behavior:
//   - If the target lies in (self, successor], return the successor (lookup ends).
//   - Otherwise, check whether currentI ∈ (self, successor]:
//   - If yes, use the de Bruijn list to forward towards the correct next imaginary node predecessor.
//     Each candidate node is tried in reverse order (from closest to farthest).
//     If all fail, fallback to the immediate successor.
//   - If not, forward directly to the successor (this node is not the predecessor of currentI).
//
// Errors:
//   - Returns an error if the routing table is not initialized (successor is nil).
//   - Returns an error if arithmetic (MulKMod, AddMod, NextDigitBaseK) fails.
//   - Returns ctx.Err() if the context has expired or been canceled.
func (n *Node) FindSuccessorStep(ctx context.Context, target, currentI, kshift domain.ID) (*domain.Node, error) {
	// Abort if context expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	self := n.rt.Self()
	succ := n.rt.FirstSuccessor()
	// check if the target is in (self, successor]
	if succ == nil {
		n.lgr.Error("FindSuccessorStep: routing table not initialized (successor is nil)")
		return nil, status.Error(codes.Internal, "routing table not initialized")
	}
	if target.Between(self.ID, succ.ID) {
		n.lgr.Debug("EndLookup: target in (self, successor], returning successor",
			logger.F("target", target.ToHexString(true)), logger.FNode("successor", succ))
		return succ, nil
	}

	// currentI is in (self, successor]: try de Bruijn routing
	if currentI.Between(self.ID, succ.ID) {

		// Compute next digit and shifted target
		nextDigit, nextKshift, err := n.rt.Space().NextDigitBaseK(kshift)
		if err != nil {
			n.lgr.Error("FindSuccessorStep: failed to compute next digit and kshift",
				logger.F("target", target.ToHexString(true)), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to compute next digit and kshift")
		}
		// Compute next imaginary node
		nextI, err := n.rt.Space().MulKMod(currentI)
		if err != nil {
			n.lgr.Error("FindSuccessorStep: failed to compute nextI (MulKMod)",
				logger.F("target", target.ToHexString(true)), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to compute nextI")
		}
		nextI, err = n.rt.Space().AddMod(nextI, n.rt.Space().FromUint64(nextDigit))
		if err != nil {
			n.lgr.Error("FindSuccessorStep: failed to compute nextI (AddMod)",
				logger.F("target", target.ToHexString(true)), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to compute nextI")
		}

		Bruijn := n.rt.DeBruijnList() // get de Bruijn list
		if Bruijn != nil && len(Bruijn) > 0 {

			if nextI.Equal(currentI) {
				n.lgr.Error("FindSuccessorStep: nextI equals currentI, potential infinite loop",
					logger.F("target", target.ToHexString(true)), logger.F("currentI", currentI.ToHexString(true)), logger.F("nextI", nextI.ToHexString(true)), logger.F("kshift", kshift.ToHexString(true)), logger.F("nextKshift", nextKshift.ToHexString(true)))
				return nil, status.Error(codes.Internal, "nextI equals currentI, potential infinite loop")
			}

			// Select de Bruijn next hop
			index := n.findNextHop(Bruijn, nextI)
			for i := index; i >= 0; i-- {
				d := Bruijn[i]
				if d == nil {
					continue
				}
				n.lgr.Debug("FindSuccessorStep: forwarding to de Bruijn node",
					logger.F("target", target.ToHexString(true)), logger.FNode("nextHop", d))
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
				}
				// Abort if context expired
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
					n.lgr.Error("FindSuccessorStep: lookup interrupted by timeout/cancel",
						logger.F("tryIdx", i), logger.F("addr", d.Addr), logger.F("err", err))
					return nil, ctx.Err()
				}
				n.lgr.Warn("FindSuccessorStep: de Bruijn hop failed, trying previous candidate",
					logger.F("tryIdx", i), logger.FNode("failedNode", d), logger.F("err", err))
			}
		}

		// Fallback: de Bruijn list empty or all failed: use successor
		n.lgr.Warn("FindSuccessorStep: de Bruijn failed or empty, falling back to successor",
			logger.F("target", target.ToHexString(true)), logger.FNode("nextHop", succ))
		cli, err := n.cp.GetFromPool(succ.Addr)
		if err != nil {
			n.lgr.Error("FindSuccessorStep: failed to get connection from pool (successor)",
				logger.F("addr", succ.Addr), logger.F("err", err))
			return nil, status.Error(codes.Internal, "failed to get connection to successor")
		}
		return client.FindSuccessorStep(ctx, cli, n.Space(), target, nextI, nextKshift)
	}

	// Default: forward to successor
	n.lgr.Debug("FindSuccessorStep: forwarding to successor",
		logger.F("target", target.ToHexString(true)), logger.FNode("nextHop", succ))
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		n.lgr.Error("FindSuccessorStep: failed to get connection from pool for successor",
			logger.F("addr", succ.Addr), logger.F("err", err))
		return nil, status.Error(codes.Internal, "failed to get connection to successor")
	}
	return client.FindSuccessorStep(ctx, cli, n.Space(), target, currentI, kshift)
}

// Self returns the local node information.
//
// Returns:
//   - A pointer to this node's own Node structure.
func (n *Node) Self() *domain.Node {
	return n.rt.Self()
}

// Predecessor returns the current predecessor of this node.
//
// The predecessor may be nil if it has not yet been established
// (e.g., immediately after node initialization or if the node
// is temporarily disconnected).
//
// Returns:
//   - A pointer to the predecessor Node, or nil if none is set.
func (n *Node) Predecessor() *domain.Node {
	return n.rt.GetPredecessor()
}

// SuccessorList returns the current successor list of this node.
//
// The successor list provides fault tolerance by keeping track of
// multiple consecutive successors on the identifier circle.
// The first entry is the immediate successor, followed by backup
// successors in order.
//
// Returns:
//   - A slice of Node pointers representing the successor list.
//     Some entries may be nil if not yet populated.
func (n *Node) SuccessorList() []*domain.Node {
	return n.rt.SuccessorList()
}

// DeBruijnList returns the current de Bruijn list of this node.
//
// Returns:
//   - A slice of Node pointers representing the de Bruijn neighbors.
//     Some entries may be nil if not yet populated.
func (n *Node) DeBruijnList() []*domain.Node {
	return n.rt.DeBruijnList()
}

// Notify informs this node about a potential predecessor.
//
// The stabilization protocol invokes Notify(p) on a node's successor.
// If the candidate p lies in (pred, self), this node adopts p as its new
// predecessor and transfers the keys it no longer owns.
//
// Ownership rule (Chord/Koorde):
//   - This node (self) owns keys in the interval (pred, self].
//   - After updating predecessor to p (with p ∈ (pred, self)),
//     self will own (p, self], while p will own (pred, p].
//   - Therefore, on update, self must transfer keys in (pred, p] to p.
//
// Behavior:
//   - Ignores nil or self notifications.
//   - If no predecessor is set, or if p ∈ (pred, self), updates the predecessor.
//   - On update: AddRef(p), SetPredecessor(p), Release(old pred),
//     and transfer resources in (pred, p] to p.
func (n *Node) Notify(p *domain.Node) {
	self := n.rt.Self()
	// check if the notifier is nil or self
	if p == nil || p.ID.Equal(self.ID) {
		return
	}

	// get current predecessor
	pred := n.rt.GetPredecessor()

	// Update if no predecessor is set, or p is a better candidate
	if pred == nil || p.ID.Between(pred.ID, self.ID) {
		// addRef new predecessor
		if err := n.cp.AddRef(p.Addr); err != nil {
			n.lgr.Warn("Notify: failed to add new predecessor to pool",
				logger.FNode("newPredecessor", p), logger.F("err", err))
		}

		// Update routing table
		n.rt.SetPredecessor(p)

		// Release old predecessor
		if pred != nil {
			if err := n.cp.Release(pred.Addr); err != nil {
				n.lgr.Warn("Notify: failed to release old predecessor",
					logger.FNode("node", pred), logger.F("err", err))
			}
		}

		// Asynchronous resource transfer: (self.ID, p.ID]
		resources := n.s.Between(self.ID, p.ID)
		if len(resources) > 0 {
			go n.transferResourcesAsync(p, resources)
		}
		// log update
		n.lgr.Info("Notify: predecessor updated",
			logger.FNode("newPredecessor", p),
			logger.FNode("oldPredecessor", pred))
	}
}

func (n *Node) transferResourcesAsync(p *domain.Node, resources []domain.Resource) {
	ctx, cancel := context.WithTimeout(context.Background(), n.cp.FailureTimeout())
	defer cancel()
	cli, err := n.cp.GetFromPool(p.Addr)
	if err != nil {
		n.lgr.Error("transferResourcesAsync: failed to get connection to new predecessor",
			logger.FNode("predecessor", p), logger.F("err", err))
		return
	}
	failed, err := client.StoreRemote(ctx, cli, resources)
	if err != nil {
		// all resources failed
		n.lgr.Error("transferResourcesAsync: store RPC failed",
			logger.FNode("predecessor", p),
			logger.F("err", err),
			logger.F("attempted", len(resources)))
		return
	}
	// Remove successfully transferred resources from local storage
	success := make(map[string]struct{}, len(resources))
	for _, r := range resources {
		success[r.Key.ToHexString(false)] = struct{}{}
	}
	for _, r := range failed {
		delete(success, r.Key.ToHexString(false))
	}
	for _, r := range resources {
		if _, ok := success[r.Key.ToHexString(false)]; ok {
			_ = n.s.Delete(r.Key)
		}
	}
	if len(failed) > 0 {
		n.lgr.Warn("transferResourcesAsync: some resources failed to transfer",
			logger.FNode("predecessor", p),
			logger.F("failedCount", len(failed)),
			logger.F("total", len(resources)))
	} else {
		n.lgr.Info("transferResourcesAsync: transfer resources to new predecessor", logger.F("count", len(resources)), logger.FNode("predecessor", p))
	}
}

// Put stores a resource in the DHT on behalf of an external client.
//
// Behavior:
//   - Validates the context (propagating client timeouts/cancellations).
//   - Locates the successor node responsible for the resource key.
//   - If this node is the successor, stores the resource locally.
//   - Otherwise, forwards the request to the responsible successor.
//
// Errors:
//   - Propagates context errors (canceled/deadline exceeded).
//   - Returns wrapped errors for lookup failures, missing successors,
//     connection pool issues, or store failures.
func (n *Node) Put(ctx context.Context, res domain.Resource) error {
	// Abort if context already canceled/expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return err
	}
	// Find the successor node responsible for this key
	succ, err := n.FindSuccessorInit(ctx, res.Key)
	if err != nil {
		return fmt.Errorf("put: failed to find successor for key %s: %w", res.RawKey, err)
	}
	if succ == nil {
		return fmt.Errorf("put: no successor found for key %s", res.RawKey)
	}

	// If this node is the successor, store locally
	if succ.ID.Equal(n.rt.Self().ID) {
		if err := n.StoreLocal(ctx, res); err != nil {
			n.lgr.Error("Put: failed to store resource locally",
				logger.F("key", res.RawKey), logger.F("err", err))
			return fmt.Errorf("put: failed to store resource locally: %w", err)
		}
		n.lgr.Info("Put: resource stored locally",
			logger.F("key", res.RawKey))
		return nil
	}

	// Otherwise, forward the resource to the successor
	sres := []domain.Resource{res}
	cli, err := n.cp.GetFromPool(succ.Addr)
	var econn *grpc.ClientConn
	if err != nil {
		// create an ephimeral connection
		cli, econn, err = n.cp.DialEphemeral(succ.Addr)
		if err != nil {
			n.lgr.Error("Put: failed to get connection to successor",
				logger.F("key", res.RawKey), logger.FNode("successor", succ), logger.F("err", err))
			return fmt.Errorf("put: failed to get connection to successor %s: %w", succ.Addr, err)
		}
		defer econn.Close()
	}
	if _, err := client.StoreRemote(ctx, cli, sres); err != nil {
		n.lgr.Error("Put: failed to store resource at successor",
			logger.F("key", res.RawKey), logger.FNode("successor", succ), logger.F("err", err))
		return fmt.Errorf("put: failed to store resource at successor %s: %w", succ.Addr, err)
	}
	// Success
	n.lgr.Info("Put: resource stored at successor",
		logger.F("key", res.RawKey), logger.FNode("successor", succ))
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
	// Abort if context already canceled/expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Find the successor node responsible for this key
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
			n.lgr.Error("Get: failed to retrieve resource locally",
				logger.F("key", id.ToHexString(true)), logger.F("err", err))
			return nil, fmt.Errorf("get: failed to retrieve resource locally: %w", err)
		}
		return &res, nil
	}

	// Otherwise, forward the request to the successor
	var econn *grpc.ClientConn
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		// fallback: create ephemeral connection
		cli, econn, err = n.cp.DialEphemeral(succ.Addr)
		if err != nil {
			n.lgr.Error("Get: failed to get connection to successor",
				logger.F("key", id.ToHexString(true)), logger.FNode("successor", succ), logger.F("err", err))
			return nil, fmt.Errorf("get: failed to get connection to successor %s: %w", succ.Addr, err)
		}
		defer econn.Close()
	}
	res, err := client.RetrieveRemote(ctx, cli, n.Space(), id)
	if err != nil {
		n.lgr.Error("Get: failed to retrieve resource from successor",
			logger.F("key", id.ToHexString(true)), logger.FNode("successor", succ), logger.F("err", err))
		return nil, fmt.Errorf("get: failed to retrieve resource from successor %s: %w", succ.Addr, err)
	}

	// Success
	n.lgr.Info("Get: resource retrieved from successor",
		logger.F("key", id.ToHexString(true)), logger.FNode("successor", succ))
	return res, nil
}

// Delete removes a resource from the DHT on behalf of an external client.
//
// Behavior:
//   - Validates the context.
//   - Locates the successor responsible for the given key.
//   - If this node is the successor, deletes the resource locally.
//   - Otherwise, forwards the request to the successor.
//
// Returns:
//   - nil if the resource was deleted successfully.
//   - status.Error(codes.NotFound, ...) if the resource does not exist.
//   - error for routing or RPC failures.
func (n *Node) Delete(ctx context.Context, id domain.ID) error {
	// Abort if context already canceled/expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return err
	}

	// Find successor
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
			n.lgr.Error("Delete: failed to delete resource locally",
				logger.F("key", id.ToHexString(true)), logger.F("err", err))
			return fmt.Errorf("delete: failed to delete resource locally: %w", err)
		}
		n.lgr.Info("Delete: resource deleted locally",
			logger.F("key", id.ToHexString(true)))
		return nil
	}
	// Otherwise, forward the request to the successor
	var econn *grpc.ClientConn
	cli, err := n.cp.GetFromPool(succ.Addr)
	if err != nil {
		// fallback: create ephemeral connection
		cli, econn, err = n.cp.DialEphemeral(succ.Addr)
		if err != nil {
			n.lgr.Error("Delete: failed to get connection to successor",
				logger.F("key", id.ToHexString(true)), logger.FNode("successor", succ), logger.F("err", err))
			return fmt.Errorf("delete: failed to get connection to successor %s: %w", succ.Addr, err)
		}
		defer econn.Close()
	}
	if err := client.RemoveRemote(ctx, cli, id); err != nil {
		n.lgr.Error("Delete: failed to delete resource at successor",
			logger.F("key", id.ToHexString(true)), logger.FNode("successor", succ), logger.F("err", err))
		return fmt.Errorf("delete: failed to delete resource at successor %s: %w", succ.Addr, err)
	}
	n.lgr.Info("Delete: resource deleted at successor",
		logger.F("key", id.ToHexString(true)), logger.FNode("successor", succ))
	return nil
}

// StoreLocal stores the given resource in the local node's storage.
// This method is invoked in the node-to-node path (via StoreRemote).
//
// Behavior:
//   - If this node has no predecessor (bootstrap phase), it considers
//     itself responsible for all keys and stores the resource.
//   - If the resource key ∈ (pred, self], the resource is stored locally.
//   - Otherwise, this node is not responsible and returns an error
//     (the caller must retry the lookup and forward correctly).
func (n *Node) StoreLocal(ctx context.Context, resource domain.Resource) error {
	// Abort if context already canceled/expired
	if err := ctxutil.CheckContext(ctx); err != nil {
		return err
	}

	pred := n.rt.GetPredecessor()
	// If no predecessor or key in (pred, self], store locally
	if pred == nil || resource.Key.Between(pred.ID, n.rt.Self().ID) {
		n.s.Put(resource)
		return nil
	}
	// Not responsible: return error
	return fmt.Errorf("storelocal: not responsible for key %s", resource.RawKey)
}

// RetrieveLocal fetches a resource from the local storage by its identifier.
// This method is invoked in the node-to-node path (via RetrieveRemote).
//
// Behavior:
//   - Looks up the given ID in the local storage.
//   - Returns the resource if found.
//   - Returns domain.ErrResourceNotFound if the resource does not exist.
//
// Note: Unlike Get (client-facing), this method does not perform routing.
// It only checks the local storage of this node.
func (n *Node) RetrieveLocal(id domain.ID) (domain.Resource, error) {
	return n.s.Get(id)
}

// RemoveLocal deletes a resource from the local storage by its identifier.
// This method is invoked in the node-to-node path (via DeleteRemote).
//
// Behavior:
//   - Attempts to delete the resource with the given ID from local storage.
//   - Returns nil if the resource was successfully removed.
//   - Returns domain.ErrResourceNotFound if the resource does not exist.
//
// Note: Unlike Delete (client-facing), this method does not perform routing.
// It only operates on the local storage of this node.
func (n *Node) RemoveLocal(id domain.ID) error {
	return n.s.Delete(id)
}

// GetAllResourceStored returns a snapshot of all resources currently
// stored in this node's local storage.
//
// Behavior:
//   - Retrieves the full list of resources maintained by the node.
//   - The returned slice is a copy/snapshot: subsequent modifications
//     to the storage are not reflected in the result.
//
// Intended use:
//   - Debugging and monitoring (e.g., inspecting storage state).
//
// Note: This method does not perform any routing. It only exposes the
// resources owned by the local storage of this node.
func (n *Node) GetAllResourceStored() []domain.Resource {
	return n.s.All()
}

// LookUp performs a DHT lookup for the given identifier and returns
// the successor node responsible for it.
//
// Behavior:
//   - Validates the context (propagating client deadlines/cancellations).
//   - Runs a FindSuccessor lookup starting from this node.
//   - Returns the successor node if found.
//
// Returns:
//   - *domain.Node if a successor is found.
//   - error if the lookup fails or no successor can be determined.
//
// Note: This method only locates the node responsible for the given ID.
// It does not retrieve or modify any resource stored in the DHT.
func (n *Node) LookUp(ctx context.Context, id domain.ID) (*domain.Node, error) {
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	succ, err := n.FindSuccessorInit(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("lookup: failed to find successor for key %s: %w", id.ToHexString(true), err)
	}
	if succ == nil {
		return nil, fmt.Errorf("lookup: no successor found for key %s", id.ToHexString(true))
	}

	return succ, nil
}

// HandleLeave processes a graceful leave notification from a predecessor.
//
// Behavior:
//   - If the leaving node is nil or not the current predecessor, the
//     leave is ignored (benign event).
//   - If it matches the current predecessor, the predecessor pointer
//     is cleared and the connection is released from the pool.
//   - Logs at INFO level on successful removal, WARN if the release fails.
//
// Returns:
//   - nil if the leave was processed or safely ignored.
//   - error only if the input was invalid.
func (n *Node) HandleLeave(leaveNode *domain.Node) error {
	pred := n.rt.GetPredecessor()
	if leaveNode == nil || pred == nil || !leaveNode.ID.Equal(pred.ID) {
		n.lgr.Warn("HandleLeave: ignoring leave for nil or non-predecessor node",
			logger.FNode("leavingNode", leaveNode))
		return nil
	}

	// Remove predecessor
	n.rt.SetPredecessor(nil)

	// Release connection from pool
	if err := n.cp.Release(leaveNode.Addr); err != nil {
		n.lgr.Warn("HandleLeave: failed to release leaving node from pool",
			logger.FNode("leavingNode", leaveNode), logger.F("err", err))
	}

	n.lgr.Info("HandleLeave: node removed from routing table and connection pool",
		logger.FNode("leavingNode", leaveNode))
	return nil
}
