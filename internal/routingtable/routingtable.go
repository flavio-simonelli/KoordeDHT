package routingtable

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"

	"sync"
)

// routingEntry rappresenta una entry della tabella di routing, include il nodo di dominio e può essere estesa in futuro.
type routingEntry struct {
	node *domain.Node
	mu   sync.RWMutex // mutex per sincronizzare l'accesso al campo node
}

// RoutingTable rappresenta la tabella di routing di un nodo nella rete Koorde.
type RoutingTable struct {
	logger        logger.Logger   // logger per le operazioni della tabella di routing
	space         *domain.Space   // spazio degli ID e grado del grafo De Bruijn
	self          *domain.Node    // nodo che possiede la tabella di routing
	successorList []*routingEntry // log(n) successori per tolleranza ai guasti
	predecessor   *routingEntry   // predecessore nel ring
	deBruijn      []*routingEntry // finestra: anchor = predecessor(k*m), succ^1(anchor), ..., succ^k(anchor)
}

// New crea una nuova tabella di routing per il nodo specificato.
// Restituisce un puntatore alla nuova RoutingTable.
func New(self *domain.Node, space *domain.Space, succListSize int, opts ...Option) *RoutingTable {
	rt := &RoutingTable{
		self:          self,
		space:         space,
		successorList: make([]*routingEntry, succListSize),     // lista dei successori inizializzata a nil
		predecessor:   &routingEntry{},                         // inizializzato a nil
		deBruijn:      make([]*routingEntry, space.GraphGrade), // predecessor(km) + k-1 successori settati a nil
		logger:        &logger.NopLogger{},                     // default: nessun log
	}
	for i := range rt.successorList {
		rt.successorList[i] = &routingEntry{} // node = nil, mutex pronto
	}
	for i := range rt.deBruijn {
		rt.deBruijn[i] = &routingEntry{} // idem per i puntatori de Bruijn
	}
	// applica le opzioni
	for _, opt := range opts {
		opt(rt)
	}
	return rt
}

// InitSingleNode configura la tabella come rete di un solo nodo.
// Tutti i puntatori (successori, predecessore, de Bruijn) puntano alla stessa routingEntry.
func (rt *RoutingTable) InitSingleNode() {
	single := &routingEntry{node: rt.self}

	for i := range rt.successorList {
		rt.successorList[i] = single
	}
	rt.predecessor = single
	for i := range rt.deBruijn {
		rt.deBruijn[i] = single
	}

	rt.logger.Info("routing table initialized for single-node network")
}

// Space restituisce lo spazio degli ID e il grado del grafo De Bruijn.
func (rt *RoutingTable) Space() domain.Space {
	return *rt.space
}

// Self restituisce il nodo locale.
func (rt *RoutingTable) Self() *domain.Node {
	return rt.self
}

// GetSuccessor ritorna l'i-esimo successore dalla lista.
// Se l'indice è fuori range o la voce è nil, restituisce nil.
func (rt *RoutingTable) GetSuccessor(i int) *domain.Node {
	if i < 0 || i >= len(rt.successorList) {
		return nil
	}
	entry := rt.successorList[i]
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.node
}

// FirstSuccessor ritorna il primo successore (successore principale).
func (rt *RoutingTable) FirstSuccessor() *domain.Node {
	return rt.GetSuccessor(0)
}

// SetSuccessor aggiorna l'i-esimo successore con il nodo specificato.
// Se l'indice è fuori range, non fa nulla.
func (rt *RoutingTable) SetSuccessor(i int, node *domain.Node) {
	if i < 0 || i >= len(rt.successorList) {
		rt.logger.Warn("SetSuccessor index out of range", logger.F("index", i), logger.F("max", len(rt.successorList)-1))
		return
	}
	entry := rt.successorList[i]
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.node = node
}

// SuccessorList restituisce una copia della lista dei successori come slice di *domain.Node.
// Le entry nil vengono mantenute come nil.
func (rt *RoutingTable) SuccessorList() []*domain.Node {
	out := make([]*domain.Node, len(rt.successorList))
	for i, entry := range rt.successorList {
		entry.mu.RLock()
		out[i] = entry.node
		entry.mu.RUnlock()
	}
	return out
}

// GetPredecessor restituisce il predecessore del nodo.
// Se non è stato impostato, restituisce nil.
func (rt *RoutingTable) GetPredecessor() *domain.Node {
	rt.predecessor.mu.RLock()
	defer rt.predecessor.mu.RUnlock()
	return rt.predecessor.node
}

// SetPredecessor aggiorna il predecessore con il nodo specificato.
func (rt *RoutingTable) SetPredecessor(node *domain.Node) {
	rt.predecessor.mu.Lock()
	defer rt.predecessor.mu.Unlock()
	rt.predecessor.node = node
}

// GetDeBruijn ritorna il puntatore de Bruijn per la cifra digit.
// Se digit è fuori range, restituisce nil.
func (rt *RoutingTable) GetDeBruijn(digit int) *domain.Node {
	if digit < 0 || digit >= len(rt.deBruijn) {
		return nil
	}
	entry := rt.deBruijn[digit]
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	return entry.node
}

// SetDeBruijn aggiorna il puntatore de Bruijn per la cifra digit.
// Se digit è fuori range, non fa nulla.
func (rt *RoutingTable) SetDeBruijn(digit int, node *domain.Node) {
	if digit < 0 || digit >= len(rt.deBruijn) {
		rt.logger.Warn("SetDeBruijn index out of range", logger.F("index", digit), logger.F("max", len(rt.deBruijn)-1))
		return
	}
	entry := rt.deBruijn[digit]
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.node = node
}

// DeBruijnList restituisce una snapshot dei puntatori de Bruijn come slice di *domain.Node.
// Le posizioni non inizializzate restano nil.
func (rt *RoutingTable) DeBruijnList() []*domain.Node {
	out := make([]*domain.Node, len(rt.deBruijn))
	for i, entry := range rt.deBruijn {
		entry.mu.RLock()
		out[i] = entry.node
		entry.mu.RUnlock()
	}
	return out
}
