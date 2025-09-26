package routingtable

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"fmt"
	"sync"
)

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
	succListSize  int             // configured size of the successor list
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
func New(self *domain.Node, space domain.Space, succListSize int, opts ...Option) *RoutingTable {
	rt := &RoutingTable{
		self:          self,
		space:         space,
		successorList: make([]*routingEntry, succListSize),     // successors initially nil
		succListSize:  succListSize,                            // configured size of the successor list
		predecessor:   &routingEntry{},                         // predecessor initially nil
		deBruijn:      make([]*routingEntry, space.GraphGrade), // base-k de Bruijn window initially nil
		logger:        &logger.NopLogger{},                     // default: no logging
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
	// Log the creation of the routing table.
	rt.logger.Debug("routing table initialized")
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
	rt.successorList[0] = &routingEntry{node: rt.self}
	rt.predecessor = &routingEntry{node: rt.self}
	rt.deBruijn[0] = &routingEntry{node: rt.self}
	// log the reinitialization
	rt.logger.Debug("Routing table setted to single-node in dht configuration")
}

// Space return the space configuration of the koorde network.
func (rt *RoutingTable) Space() domain.Space {
	return rt.space
}

// Self returns the local node owning this routing table.
func (rt *RoutingTable) Self() *domain.Node {
	return rt.self
}

// SuccListSize returns the configured size of the successor list.
func (rt *RoutingTable) SuccListSize() int {
	return rt.succListSize
}

// GetSuccessor returns the i-th successor from the successor list.
//
// If the index is out of range or the entry does not contain a node,
// the method returns nil. Access is synchronized using a read lock
// to ensure thread-safe concurrent access.
func (rt *RoutingTable) GetSuccessor(i int) *domain.Node {
	if i < 0 || i >= len(rt.successorList) {
		rt.logger.Warn(
			"GetSuccessor: index out of range",
			logger.F("requested", i),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", len(rt.successorList)-1)),
		)
		return nil
	}
	entry := rt.successorList[i]
	entry.mu.RLock()
	node := entry.node
	entry.mu.RUnlock()
	rt.logger.Debug("GetSuccessor: returning successor", logger.F("index", i), logger.FNode("successor", node))
	return node
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
// The update is synchronized with a write lock to ensure thread-safe
// concurrent modifications.
func (rt *RoutingTable) SetSuccessor(i int, node *domain.Node) {
	if i < 0 || i >= len(rt.successorList) {
		rt.logger.Warn(
			"SetSuccessor: index out of range",
			logger.F("requested", i),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", len(rt.successorList)-1)),
		)
		return
	}
	entry := rt.successorList[i]
	entry.mu.Lock()
	entry.node = node
	entry.mu.Unlock()
	rt.logger.Debug("SetSuccessor: updated successor", logger.F("index", i), logger.FNode("successor", node))
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
	snapshot := make([]*domain.Node, 0, len(rt.successorList))
	// lock phase: take a snapshot of the successor list
	for _, entry := range rt.successorList {
		entry.mu.RLock()
		node := entry.node
		entry.mu.RUnlock()

		snapshot = append(snapshot, node)
		if node != nil {
			out = append(out, node)
		}
	}
	// debug logging phase: log the full snapshot including nils
	nodesInfo := make([]map[string]any, 0, len(snapshot))
	for i, n := range snapshot {
		if n == nil {
			nodesInfo = append(nodesInfo, map[string]any{
				"index": i,
				"node":  nil,
			})
		} else {
			nodesInfo = append(nodesInfo, map[string]any{
				"index": i,
				"id":    n.ID.String(),
				"addr":  n.Addr,
			})
		}
	}
	rt.logger.Debug("SuccessorList snapshot", logger.F("entries", nodesInfo))
	return out
}

// SetSuccessorList replaces the entire successor list with the given slice.
//
// The provided slice must have the same length as the internal successor list.
// Each entry is updated under a write lock to ensure thread safety.
// If the slice length does not match, the method logs a warning and does nothing.
func (rt *RoutingTable) SetSuccessorList(nodes []*domain.Node) {
	if len(nodes) != len(rt.successorList) {
		rt.logger.Warn(
			"SetSuccessorList: length mismatch",
			logger.F("expected", len(rt.successorList)),
			logger.F("got", len(nodes)),
		)
		return
	}
	for i, node := range nodes {
		rt.SetSuccessor(i, node)
	}
	// log
	entriesInfo := make([]map[string]any, 0, len(nodes))
	for i, node := range nodes {
		rt.SetSuccessor(i, node)

		if node == nil {
			entriesInfo = append(entriesInfo, map[string]any{
				"index": i,
				"node":  nil,
			})
		} else {
			entriesInfo = append(entriesInfo, map[string]any{
				"index": i,
				"id":    node.ID.String(),
				"addr":  node.Addr,
			})
		}
	}
	rt.logger.Debug("SetSuccessorList: successor list updated",
		logger.F("entries", entriesInfo),
	)
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
	if i <= 0 || i >= rt.succListSize {
		rt.logger.Warn(
			"PromoteCandidate: invalid index",
			logger.F("requested", i),
			logger.F("valid_range", fmt.Sprintf("[1..%d]", rt.succListSize-1)),
		)
		return
	}
	candidate := rt.GetSuccessor(i)
	if candidate == nil {
		rt.logger.Warn(
			"PromoteCandidate: candidate is nil",
			logger.F("index", i),
		)
		return
	}
	// Build a new list: candidate + all successors after it
	newList := make([]*domain.Node, 0, rt.succListSize)
	newList = append(newList, candidate)
	for j := i + 1; j < rt.succListSize; j++ {
		if succ := rt.GetSuccessor(j); succ != nil {
			newList = append(newList, succ)
		}
	}
	// Pad the list with nil to reach the configured size
	for len(newList) < rt.succListSize {
		newList = append(newList, nil)
	}
	rt.SetSuccessorList(newList)
	// Log the promotion
	rt.logger.Debug(
		"PromoteCandidate: successor promoted",
		logger.F("from_index", i),
		logger.FNode("candidate", candidate),
	)
}

// GetPredecessor return the current predecessor node.
// If the predecessor is not set, it returns nil.
// Access is synchronized with a read lock for thread safety.
func (rt *RoutingTable) GetPredecessor() *domain.Node {
	rt.predecessor.mu.RLock()
	node := rt.predecessor.node
	rt.predecessor.mu.RUnlock()
	rt.logger.Debug(
		"GetPredecessor: predecessor retrieved",
		logger.FNode("predecessor", node),
	)
	return node
}

// SetPredecessor updates the predecessor pointer to the specified node.
// Access is synchronized with a write lock to ensure thread-safe updates.
func (rt *RoutingTable) SetPredecessor(node *domain.Node) {
	rt.predecessor.mu.Lock()
	rt.predecessor.node = node
	rt.predecessor.mu.Unlock()
	rt.logger.Debug(
		"SetPredecessor: predecessor updated",
		logger.FNode("predecessor", node),
	)
}

// GetDeBruijn returns the node pointer stored in the de Bruijn entry
// corresponding to the given digit.
//
// If digit is out of range, the method returns nil. Access is synchronized
// with a read lock to ensure thread-safe concurrent access.
func (rt *RoutingTable) GetDeBruijn(digit int) *domain.Node {
	if digit < 0 || digit >= len(rt.deBruijn) {
		rt.logger.Warn(
			"GetDeBruijn: digit out of range",
			logger.F("requested", digit),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", len(rt.deBruijn)-1)),
		)
		return nil
	}
	entry := rt.deBruijn[digit]
	entry.mu.RLock()
	node := entry.node
	entry.mu.RUnlock()
	rt.logger.Debug(
		"GetDeBruijn: node retrieved",
		logger.F("digit", digit),
		logger.FNode("node", node),
	)
	return node
}

// SetDeBruijn updates the de Bruijn entry for the given digit with the specified node.
//
// If digit is out of range, the method logs a warning and does nothing.
// The update is synchronized with a write lock to ensure thread-safe
// concurrent modifications.
func (rt *RoutingTable) SetDeBruijn(digit int, node *domain.Node) {
	if digit < 0 || digit >= len(rt.deBruijn) {
		rt.logger.Warn(
			"SetDeBruijn: index out of range",
			logger.F("requested", digit),
			logger.F("valid_range", fmt.Sprintf("[0..%d]", len(rt.deBruijn)-1)),
		)
		return
	}
	entry := rt.deBruijn[digit]
	entry.mu.Lock()
	entry.node = node
	entry.mu.Unlock()
	rt.logger.Debug(
		"SetDeBruijn: entry updated",
		logger.F("digit", digit),
		logger.FNode("node", node),
	)
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
	snapshot := make([]*domain.Node, 0, len(rt.deBruijn))
	// snapshot phase: read all entries under lock
	for _, entry := range rt.deBruijn {
		entry.mu.RLock()
		node := entry.node
		entry.mu.RUnlock()

		snapshot = append(snapshot, node)
		if node != nil {
			out = append(out, node)
		}
	}
	// logging phase: log the full snapshot including nils
	nodesInfo := make([]map[string]any, 0, len(snapshot))
	for i, n := range snapshot {
		if n == nil {
			nodesInfo = append(nodesInfo, map[string]any{
				"digit": i,
				"node":  nil,
			})
		} else {
			nodesInfo = append(nodesInfo, map[string]any{
				"digit": i,
				"id":    n.ID.String(),
				"addr":  n.Addr,
			})
		}
	}
	rt.logger.Debug("DeBruijnList snapshot", logger.F("entries", nodesInfo))
	return out
}

// SetDeBruijnList overwrites the entire de Bruijn window with the provided nodes.
// The input slice must have a length equal to the routing table's GraphGrade.
//
// For each position in the slice:
//   - If the element is nil, the corresponding entry is cleared.
//   - If the element is non-nil, it is written as the new pointer.
//
// Each entry is updated under its own lock, ensuring thread-safe writes.
// After updating, the method emits a DEBUG-level log with the new state of
// the de Bruijn list, including indices and nil entries, for debugging
// and monitoring purposes.
//
// This method does not modify the size of the list; it only replaces its contents.
func (rt *RoutingTable) SetDeBruijnList(nodes []*domain.Node) {
	if len(nodes) != len(rt.deBruijn) {
		rt.logger.Warn(
			"SetDeBruijnList: length mismatch",
			logger.F("expected", len(rt.deBruijn)),
			logger.F("got", len(nodes)),
		)
		return
	}
	// update phase: write each entry under its lock
	for i := 0; i < len(rt.deBruijn); i++ {
		rt.deBruijn[i].mu.Lock()
		rt.deBruijn[i].node = nodes[i]
		rt.deBruijn[i].mu.Unlock()
	}
	// logging phase: log the new state including nils
	entriesInfo := make([]map[string]any, 0, len(nodes))
	for i, n := range nodes {
		if n == nil {
			entriesInfo = append(entriesInfo, map[string]any{
				"digit": i,
				"node":  nil,
			})
		} else {
			entriesInfo = append(entriesInfo, map[string]any{
				"digit": i,
				"id":    n.ID.String(),
				"addr":  n.Addr,
			})
		}
	}
	rt.logger.Debug("SetDeBruijnList: list updated",
		logger.F("entries", entriesInfo),
	)
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
//
// This method is intended for debugging and monitoring purposes.
// It does not modify the routing table and can be safely invoked
// concurrently with other operations.
func (rt *RoutingTable) DebugLog() {
	// self
	self := rt.self

	// predecessor
	rt.predecessor.mu.RLock()
	pred := rt.predecessor.node
	rt.predecessor.mu.RUnlock()

	// successors snapshot
	successors := make([]map[string]any, 0, len(rt.successorList))
	for i, entry := range rt.successorList {
		entry.mu.RLock()
		node := entry.node
		entry.mu.RUnlock()
		if node == nil {
			successors = append(successors, map[string]any{"index": i, "node": nil})
		} else {
			successors = append(successors, map[string]any{"index": i, "id": node.ID.String(), "addr": node.Addr})
		}
	}

	// de Bruijn snapshot
	debruijn := make([]map[string]any, 0, len(rt.deBruijn))
	for i, entry := range rt.deBruijn {
		entry.mu.RLock()
		node := entry.node
		entry.mu.RUnlock()
		if node == nil {
			debruijn = append(debruijn, map[string]any{"digit": i, "node": nil})
		} else {
			debruijn = append(debruijn, map[string]any{"digit": i, "id": node.ID.String(), "addr": node.Addr})
		}
	}

	rt.logger.Debug("RoutingTable snapshot",
		logger.FNode("self", self),
		logger.FNode("predecessor", pred),
		logger.F("successors", successors),
		logger.F("debruijn", debruijn),
	)
}
