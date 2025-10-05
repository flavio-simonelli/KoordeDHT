package routingtable

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"fmt"
	"sync"
)

// ----------------------------------------------------------------
// Routing Entry
// ----------------------------------------------------------------

// routingEntry represents a single entry in the routing table.
//
// Each entry holds a reference to a domain.Node and provides
// thread-safe access through a read/write mutex. The type is
// defined as a struct to allow future extensions (e.g., storing
// metadata, timestamps, or health information about the node).
type routingEntry struct {
	// node is the domain-level node stored in this entry.
	// It can be read and updated concurrently using mu.
	node *domain.Node

	// mu synchronizes access to node, ensuring safe
	// concurrent reads and writes.
	mu sync.RWMutex
}

// Set updates the routing entry with the given node.
// The update is protected by a write lock to ensure thread-safe modifications.
//
// If n is nil, the entry will be cleared.
func (e *routingEntry) Set(n *domain.Node) {
	e.mu.Lock()
	e.node = n
	e.mu.Unlock()
}

// Get retrieves the current node stored in the routing entry.
// The read is protected by a read lock to allow concurrent access.
//
// Returns nil if no node is currently set.
func (e *routingEntry) Get() *domain.Node {
	e.mu.RLock()
	n := e.node
	e.mu.RUnlock()
	return n
}

// ----------------------------------------------------------------
// Routing Table
// ----------------------------------------------------------------

// RoutingTable represents the routing state of a node in the Koorde DHT.
//
// A routing table combines ring-based Chord-like links (successor/predecessor)
// with De Bruijn graph links, enabling efficient lookups while ensuring
// fault tolerance. It is owned by a single node (self) and maintained
// through stabilization protocols.
//
// Fields:
//   - logger: used for structured logging of routing operations.
//   - space: identifier space configuration (bit-length and graph degree).
//   - self: the local node that owns this routing table.
//   - successorList: a list of O(log n) successors, providing redundancy
//     and fault tolerance against node failures.
//   - predecessor: the immediate predecessor of this node on the ring.
//   - deBruijn: the De Bruijn window (routing entries anchored at
//     predecessor(k*m), followed by successors that simulate base-k
//     de Bruijn edges).
type RoutingTable struct {
	logger        logger.Logger   // logger for routing table operations
	space         domain.Space    // identifier space and de Bruijn graph degree
	self          *domain.Node    // the local node owning this routing table
	successorList []*routingEntry // O(log n) (set by configuration) successors for fault tolerance
	predecessor   *routingEntry   // immediate predecessor in the ring
	deBruijn      []*routingEntry // de Bruijn window entries for base-k routing
}

// New creates and initializes a new RoutingTable for the given node.
//
// The routing table is initialized with empty successor entries, an empty
// predecessor entry, and a de Bruijn window of size space.GraphGrade.
// By default, logging is disabled (NopLogger) unless overridden with options.
//
// Arguments:
//   - self: the local node owning this routing table.
//   - space: the identifier space configuration (bit-length and graph degree).
//   - succListSize: the size of the successor list (typically O(log n)).
//   - opts: functional options to customize the routing table (logger).
//
// Returns:
//   - *RoutingTable: a pointer to the newly created routing table, with all
//     entries initialized but containing nil nodes until stabilization fills them.
func New(self *domain.Node, space domain.Space, opts ...Option) *RoutingTable {
	rt := &RoutingTable{
		self:          self,
		space:         space,
		successorList: make([]*routingEntry, space.SuccListSize), // successors initially nil
		predecessor:   &routingEntry{},                           // predecessor initially nil
		deBruijn:      make([]*routingEntry, space.GraphGrade),   // base-k de Bruijn window initially nil
		logger:        &logger.NopLogger{},                       // default: no logging
	}
	// Initialize successor list entries with empty routingEntry structs.
	for i := range rt.successorList {
		rt.successorList[i] = &routingEntry{}
	}
	// Initialize de Bruijn entries with empty routingEntry structs.
	for i := range rt.deBruijn {
		rt.deBruijn[i] = &routingEntry{}
	}
	// Apply functional options (custom logger).
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// InitSingleNode configures the routing table to represent a single-node network.
//
// In this configuration, all routing pointers (successor list, predecessor,
// and de Bruijn entries) point to the local node itself. This state is
// typically used when bootstrapping a fresh Koorde network with only one
// participating node.
//
// After initialization:
//   - Every successor entry points to self.
//   - The predecessor points to self.
//   - Every de Bruijn entry points to self.
func (rt *RoutingTable) InitSingleNode() {
	rt.SetSuccessor(0, rt.self)
	rt.SetPredecessor(rt.self)
	rt.SetDeBruijn(0, rt.self)
}

// Space return the space configuration of the koorde network.
func (rt *RoutingTable) Space() *domain.Space {
	return &rt.space
}

// Self returns the local node owning this routing table.
func (rt *RoutingTable) Self() *domain.Node {
	return rt.self
}

// GetSuccessor returns the i-th successor from the successor list.
//
// If the index is out of range or the entry does not contain a node,
// the method returns nil. The underlying routingEntry manages its own
// synchronization to ensure thread-safe concurrent access.
func (rt *RoutingTable) GetSuccessor(i int) *domain.Node {
	if i < 0 || i >= rt.Space().SuccListSize {
		rt.logger.Warn(
			"GetSuccessor: index out of range",
			logger.F("requested", i),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", rt.Space().SuccListSize-1)),
		)
		return nil
	}
	return rt.successorList[i].Get()
}

// FirstSuccessor return the first successor in the successor list.
// It is a convenience method equivalent to GetSuccessor(0).
// If the successor list is empty or the first entry is nil, it returns nil.
func (rt *RoutingTable) FirstSuccessor() *domain.Node {
	return rt.GetSuccessor(0)
}

// SetSuccessor updates the i-th successor entry with the specified node.
//
// If the index is out of range, the method logs a warning and does nothing.
// The underlying routingEntry manages its own synchronization to ensure
// thread-safe updates.
func (rt *RoutingTable) SetSuccessor(i int, node *domain.Node) {
	if i < 0 || i >= rt.Space().SuccListSize {
		rt.logger.Warn(
			"SetSuccessor: index out of range",
			logger.F("requested", i),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", rt.Space().SuccListSize-1)),
		)
		return
	}
	rt.successorList[i].Set(node)
}

// SuccessorList returns a slice of all non-nil successors currently known
// in the routing table.
//
// Each successor entry is read under a read lock to ensure thread-safe access.
// The returned slice contains only initialized successors; entries with a nil
// node are skipped. Callers receive a shallow copy of the successor list and
// may safely modify it without affecting the internal state.
func (rt *RoutingTable) SuccessorList() []*domain.Node {
	out := make([]*domain.Node, 0, len(rt.successorList))
	for _, entry := range rt.successorList {
		node := entry.Get()
		if node != nil {
			out = append(out, node)
		}
	}
	return out
}

// SetSuccessorList replaces the successor list with the given slice.
//
// Behavior:
//   - If len(nodes) > len(successorList), extra nodes are truncated.
//   - If len(nodes) < len(successorList), missing entries are set to nil.
//
// Each entry is updated under a write lock on the individual routing entries.
func (rt *RoutingTable) SetSuccessorList(nodes []*domain.Node) {
	expected := rt.Space().SuccListSize

	if len(nodes) > expected {
		rt.logger.Warn(
			"SetSuccessorList: truncating input slice",
			logger.F("expected", expected),
			logger.F("got", len(nodes)),
		)
		nodes = nodes[:expected]
	}

	// fill entries with provided nodes
	for i, node := range nodes {
		rt.SetSuccessor(i, node)
	}

	// pad with nil if input shorter than expected
	for i := len(nodes); i < expected; i++ {
		rt.SetSuccessor(i, nil)
	}
}

// PromoteCandidate restructures the successor list by promoting the
// successor at position i to the head of the list.
//
// Behavior:
//   - The node at index i becomes the new successor at position 0.
//   - All successors after position i are shifted forward,
//     preserving their relative order.
//   - All successors before position i are discarded.
//   - The list is padded with nil entries until it reaches
//     the configured successor list size.
//
// Parameters:
//   - i: the index of the candidate successor to promote.
//     If i <= 0 or out of range, the function does nothing.
func (rt *RoutingTable) PromoteCandidate(i int) {
	expected := rt.Space().SuccListSize
	if i <= 0 || i >= expected {
		rt.logger.Warn(
			"PromoteCandidate: invalid index",
			logger.F("requested", i),
			logger.F("valid_range", fmt.Sprintf("[1..%d]", expected-1)),
		)
		return
	}
	candidate := rt.successorList[i].Get()
	if candidate == nil {
		rt.logger.Warn(
			"PromoteCandidate: candidate is nil",
			logger.F("index", i),
		)
		return
	}

	// Build a new list: candidate + successors after it
	newList := make([]*domain.Node, expected)
	newList[0] = candidate
	k := 1
	for j := i + 1; j < expected; j++ {
		if succ := rt.successorList[j].Get(); succ != nil {
			newList[k] = succ
			k++
		}
	}
	// remaining slots stay nil
	rt.SetSuccessorList(newList)
	// log the promotion
	rt.logger.Debug(
		"PromoteCandidate: successor promoted",
		logger.F("from_index", i),
		logger.FNode("candidate", candidate),
	)

}

// GetPredecessor returns the current predecessor node.
//
// If the predecessor is not set, it returns nil.
// The underlying routingEntry manages its own synchronization
// to ensure thread-safe access.
func (rt *RoutingTable) GetPredecessor() *domain.Node {
	return rt.predecessor.Get()
}

// SetPredecessor updates the predecessor pointer to the specified node.
//
// The underlying routingEntry manages its own synchronization
// to ensure thread-safe updates.
func (rt *RoutingTable) SetPredecessor(node *domain.Node) {
	rt.predecessor.Set(node)
}

// GetDeBruijn returns the node pointer stored in the de Bruijn entry
// corresponding to the given digit.
//
// If digit is out of range, the method returns nil.
// The underlying routingEntry manages its own synchronization
// to ensure thread-safe concurrent access.
func (rt *RoutingTable) GetDeBruijn(digit int) *domain.Node {
	if digit < 0 || digit >= len(rt.deBruijn) {
		rt.logger.Warn(
			"GetDeBruijn: digit out of range",
			logger.F("requested", digit),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", len(rt.deBruijn)-1)),
		)
		return nil
	}
	return rt.deBruijn[digit].Get()
}

// SetDeBruijn updates the de Bruijn entry for the given digit with the specified node.
//
// If digit is out of range, the method logs a warning and does nothing.
// The underlying routingEntry manages its own synchronization
// to ensure thread-safe updates.
func (rt *RoutingTable) SetDeBruijn(digit int, node *domain.Node) {
	if digit < 0 || digit >= len(rt.deBruijn) {
		rt.logger.Warn(
			"SetDeBruijn: index out of range",
			logger.F("requested", digit),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", len(rt.deBruijn)-1)),
		)
		return
	}
	rt.deBruijn[digit].Set(node)
}

// DeBruijnList returns a slice of all non-nil de Bruijn entries currently known
// in the routing table.
//
// Each entry is read under a read lock to ensure thread-safe access.
// The returned slice contains only initialized de Bruijn pointers; entries
// with a nil node are skipped. Callers receive a shallow copy of the data
// and may safely modify it without affecting the internal state.
//
// Example:
//
//	// If the de Bruijn window has [n1, nil, n2]
//	// the returned slice will be [n1, n2].
func (rt *RoutingTable) DeBruijnList() []*domain.Node {
	out := make([]*domain.Node, 0, len(rt.deBruijn))
	for _, entry := range rt.deBruijn {
		if node := entry.Get(); node != nil {
			out = append(out, node)
		}
	}
	return out
}

// SetDeBruijnList replaces the entire de Bruijn window with the provided slice.
//
// Behavior:
//   - If len(nodes) > len(deBruijn), extra nodes are truncated.
//   - If len(nodes) < len(deBruijn), missing entries are set to nil.
//
// Each entry is updated under a write lock on the individual routing entries.
// This method does not modify the size of the de Bruijn window.
func (rt *RoutingTable) SetDeBruijnList(nodes []*domain.Node) {
	expected := rt.Space().GraphGrade

	if len(nodes) > expected {
		rt.logger.Warn(
			"SetDeBruijnList: truncating input slice",
			logger.F("expected", expected),
			logger.F("got", len(nodes)),
		)
		nodes = nodes[:expected]
	}

	// fill entries with provided nodes
	for i, node := range nodes {
		rt.SetDeBruijn(i, node)
	}

	// pad with nil if input shorter than expected
	for i := len(nodes); i < expected; i++ {
		rt.SetDeBruijn(i, nil)
	}
}

// DebugLog emits a structured DEBUG-level log entry containing a snapshot
// of the entire routing table.
//
// Unlike calling the public getters (GetSuccessor, GetPredecessor, GetDeBruijn),
// this method accesses the internal entries directly under read locks, in order
// to avoid triggering additional per-entry debug logs. As a result, DebugLog
// produces a single compact log entry that reflects the current state without
// side effects.
//
// The snapshot includes:
//   - Self node (the node that owns this routing table)
//   - Predecessor (nil if not set)
//   - Successor list (all entries, including nils, with indices)
//   - De Bruijn list (all entries, including nils, with digits)
func (rt *RoutingTable) DebugLog() {
	self := rt.self
	pred := rt.GetPredecessor()

	// successors snapshot
	successors := make([]map[string]any, 0, len(rt.successorList))
	for i := range rt.successorList {
		if node := rt.GetSuccessor(i); node == nil {
			successors = append(successors, map[string]any{"index": i, "node": nil})
		} else {
			successors = append(successors, map[string]any{
				"index": i,
				"idhex": node.ID.ToHexString(true),
				"idbin": node.ID.ToBinaryString(true),
				"addr":  node.Addr,
			})
		}
	}

	// de Bruijn snapshot
	debruijn := make([]map[string]any, 0, len(rt.deBruijn))
	for i := range rt.deBruijn {
		if node := rt.GetDeBruijn(i); node == nil {
			debruijn = append(debruijn, map[string]any{"digit": i, "node": nil})
		} else {
			debruijn = append(debruijn, map[string]any{
				"digit": i,
				"idhex": node.ID.ToHexString(true),
				"idbin": node.ID.ToBinaryString(true),
				"addr":  node.Addr,
			})
		}
	}

	rt.logger.Debug("RoutingTable snapshot",
		logger.FNode("self", self),
		logger.FNode("predecessor", pred),
		logger.F("successors", successors),
		logger.F("debruijn", debruijn),
	)
}
