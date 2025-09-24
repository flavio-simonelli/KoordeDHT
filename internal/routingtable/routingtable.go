package routingtable

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"

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
	single := &routingEntry{node: rt.self}
	// Successor list: all entries point to self.
	rt.successorList[0] = single
	// Predecessor: points to self.
	rt.predecessor = single
	// De Bruijn window: all entries point to self.
	rt.deBruijn[0] = single
	rt.logger.Info("routing table initialized for single-node network")
}

// Space return the space configuration of the koorde network.
func (rt *RoutingTable) Space() domain.Space {
	return rt.space
}

// Self returns the local node owning this routing table.
func (rt *RoutingTable) Self() *domain.Node {
	return rt.self
}

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
		return nil
	}
	entry := rt.successorList[i]
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.node
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
			"SetSuccessor index out of range",
			logger.F("index", i),
			logger.F("max", len(rt.successorList)-1),
		)
		return
	}
	entry := rt.successorList[i]
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.node = node
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
		entry.mu.RLock()
		if entry.node != nil {
			out = append(out, entry.node)
		}
		entry.mu.RUnlock()
	}
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
}

// GetPredecessor return the current predecessor node.
// If the predecessor is not set, it returns nil.
// Access is synchronized with a read lock for thread safety.
func (rt *RoutingTable) GetPredecessor() *domain.Node {
	rt.predecessor.mu.RLock()
	defer rt.predecessor.mu.RUnlock()
	return rt.predecessor.node
}

// SetPredecessor updates the predecessor pointer to the specified node.
// Access is synchronized with a write lock to ensure thread-safe updates.
func (rt *RoutingTable) SetPredecessor(node *domain.Node) {
	rt.predecessor.mu.Lock()
	defer rt.predecessor.mu.Unlock()
	rt.predecessor.node = node
}

// GetDeBruijn returns the node pointer stored in the de Bruijn entry
// corresponding to the given digit.
//
// If digit is out of range, the method returns nil. Access is synchronized
// with a read lock to ensure thread-safe concurrent access.
func (rt *RoutingTable) GetDeBruijn(digit int) *domain.Node {
	if digit < 0 || digit >= len(rt.deBruijn) {
		return nil
	}
	entry := rt.deBruijn[digit]
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.node
}

// SetDeBruijn updates the de Bruijn entry for the given digit with the specified node.
//
// If digit is out of range, the method logs a warning and does nothing.
// The update is synchronized with a write lock to ensure thread-safe
// concurrent modifications.
func (rt *RoutingTable) SetDeBruijn(digit int, node *domain.Node) {
	if digit < 0 || digit >= len(rt.deBruijn) {
		rt.logger.Warn(
			"SetDeBruijn index out of range",
			logger.F("index", digit),
			logger.F("max", len(rt.deBruijn)-1),
		)
		return
	}
	entry := rt.deBruijn[digit]
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.node = node
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
	var out []*domain.Node
	for _, entry := range rt.deBruijn {
		entry.mu.RLock()
		if entry.node != nil {
			out = append(out, entry.node)
		}
		entry.mu.RUnlock()
	}
	return out
}
