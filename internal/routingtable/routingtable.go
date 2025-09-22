package routingtable

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"

	"errors"
	"sync"
)

var (
	InvalidDegree = errors.New("invalid graph degree")
	InvalidIDBits = errors.New("invalid ID bits")
)

type routingEntry struct {
	domain.Node
}

// RoutingTable rappresenta i link di un nodo Koorde.
type RoutingTable struct {
	logger        logger.Logger   // logger per la routing table (default: NopLogger)
	idBits        int             // numero di bit dello spazio ID
	graphGrade    int             // grado del grafo De Bruijn
	self          *routingEntry   // il nodo locale
	succMu        []sync.RWMutex  // mutex per la lista di successori
	successorList []*routingEntry // log(n) successori per tolleranza ai guasti
	predMu        sync.RWMutex    // mutex per il predecessore
	predecessor   *routingEntry   // predecessore nel ring
	dbWinMu       []sync.RWMutex
	deBruijn      []*routingEntry // finestra: anchor = predecessor(k*m), succ^1(anchor), ..., succ^k(anchor)
}

// New crea una nuova tabella di routing per il nodo specificato.
// idBits è il numero di bit dello spazio ID (es. 128).
// graphGrade è il grado del grafo De Bruijn (es. 4).
// k = log2(graphGrade) deve essere un intero positivo.
// Restituisce un puntatore alla nuova RoutingTable.
// Se graphGrade non è valido, restituisce InvalidDegree
// Se idBits non è valido, restituisce InvalidIDBits
// Inizialmente tutte le entry puntano al nodo locale; verranno aggiornate
// successivamente dalle procedure di manutenzione (fix).
func New(self domain.Node, idBits, graphGrade int, opts ...Option) (*RoutingTable, error) {
	if idBits <= 0 {
		return nil, InvalidIDBits
	}
	if graphGrade < 2 {
		return nil, InvalidDegree
	}
	rt := &RoutingTable{
		self:          &routingEntry{Node: self},
		succMu:        make([]sync.RWMutex, idBits),
		successorList: make([]*routingEntry, idBits),
		predecessor:   &routingEntry{Node: self},
		deBruijn:      make([]*routingEntry, graphGrade),
		dbWinMu:       make([]sync.RWMutex, graphGrade+1),
		logger:        &logger.NopLogger{}, // default: nessun log
	}
	// inizializza i link De Bruijn con il nodo locale
	for i := range rt.deBruijn {
		rt.deBruijn[i] = &routingEntry{Node: self}
	}
	// inizializza la successor list con il nodo locale
	for i := range rt.successorList {
		rt.successorList[i] = &routingEntry{Node: self}
	}
	// Inizializza i parametri idBits e graphGrade
	rt.idBits = idBits
	rt.graphGrade = graphGrade
	// applica le opzioni
	for _, opt := range opts {
		opt(rt)
	}
	return rt, nil
}

// Degree restituisce il grado del grafo De Bruijn.
func (rt *RoutingTable) Degree() int {
	return rt.graphGrade
}

// IDBits restituisce il numero di bit dello spazio ID.
func (rt *RoutingTable) IDBits() int {
	return rt.idBits
}

// Self restituisce il nodo locale.
func (rt *RoutingTable) Self() domain.Node {
	return rt.self.Node
}

// Successor restituisce il successore del nodo locale.
func (rt *RoutingTable) Successor(i int) (domain.Node, error) {
	if i < 0 || i >= len(rt.successorList) {
		return domain.Node{}, errors.New("index out of range")
	}
	rt.succMu[i].RLock()
	n := rt.successorList[i].Node
	rt.succMu[i].RUnlock()
	return n, nil
}

func (rt *RoutingTable) SuccessorList() []domain.Node {
	nodes := make([]domain.Node, len(rt.successorList))
	for i := range rt.successorList {
		rt.succMu[i].RLock()
		nodes[i] = rt.successorList[i].Node
		rt.succMu[i].RUnlock()
	}
	return nodes
}

// SetSuccessor aggiorna il successore del nodo locale.
func (rt *RoutingTable) SetSuccessor(i int, n domain.Node) { //TODO: mettere error
	if i < 0 || i >= len(rt.successorList) {
		return
	}
	rt.succMu[i].Lock()
	old := rt.successorList[i].Node
	rt.successorList[i] = &routingEntry{Node: n}
	rt.succMu[i].Unlock()
	if !old.ID.Equal(n.ID) {
		rt.logger.Info("routingtable.SetSuccessor",
			logger.F("old.addr", old.Addr),
			logger.F("new.addr", n.Addr),
			logger.F("old.id", old.ID.ToHexString()),
			logger.F("new.id", n.ID.ToHexString()),
		)
	}
}

// Predecessor restituisce il predecessore del nodo locale.
func (rt *RoutingTable) Predecessor() domain.Node {
	rt.predMu.RLock()
	n := rt.predecessor.Node
	rt.predMu.RUnlock()
	return n
}

// SetPredecessor aggiorna il predecessore del nodo locale.
func (rt *RoutingTable) SetPredecessor(n domain.Node) {
	rt.predMu.Lock()
	old := rt.predecessor.Node
	rt.predecessor = &routingEntry{Node: n}
	rt.predMu.Unlock()
	if !old.ID.Equal(n.ID) {
		rt.logger.Info("routingtable.SetPredecessor",
			logger.F("old.addr", old.Addr),
			logger.F("new.addr", n.Addr),
			logger.F("old.id", old.ID.ToHexString()),
			logger.F("new.id", n.ID.ToHexString()),
		)
	}
}

// DeBruijn restituisce il nodo De Bruijn all'indice specificato.
// l'indice deve essere compreso tra 0 e graphGrade.
// l'indice 0 è la radice (anchor).
// Se l'indice non è valido, restituisce errore.
func (rt *RoutingTable) DeBruijn(i int) (domain.Node, error) {
	if i < 0 || i >= len(rt.deBruijn) {
		return domain.Node{}, errors.New("index out of range")
	}
	rt.dbWinMu[i].RLock()
	n := rt.deBruijn[i].Node
	rt.dbWinMu[i].RUnlock()
	return n, nil
}

// FixDeBruijn aggiorna il nodo De Bruijn all'indice specificato.
// l'indice deve essere compreso tra 0 e graphGrade.
// l'indice 0 è la radice (anchor).
// Se l'indice non è valido, la funzione non fa nulla.
func (rt *RoutingTable) FixDeBruijn(i int, n domain.Node) {
	if i < 0 || i >= len(rt.deBruijn) {
		return
	}
	rt.dbWinMu[i].Lock()
	old := rt.deBruijn[i].Node
	rt.deBruijn[i] = &routingEntry{Node: n}
	rt.dbWinMu[i].Unlock()
	if !old.ID.Equal(n.ID) {
		rt.logger.Info("routingtable.FixDeBruijn",
			logger.F("index", i),
			logger.F("old.addr", old.Addr),
			logger.F("new.addr", n.Addr),
			logger.F("old.id", old.ID.ToHexString()),
			logger.F("new.id", n.ID.ToHexString()),
		)
	}
}

// FindPredecessor prova a trovare rapidamente il predecessore reale di id
// usando l'ancora predecessor(k·m) e la finestra dei k successori.
// FindPredecessorDB restituisce il predecessore reale di id
// usando anchor + finestra de Bruijn.
// Cerca dal k-esimo successore verso anchor.
func (rt *RoutingTable) FindPredecessor(id domain.ID) domain.Node {
	rt.dbWinMu[len(rt.deBruijn)-1].RLock()
	next := rt.deBruijn[len(rt.deBruijn)-1].Node
	rt.dbWinMu[len(rt.deBruijn)-1].RUnlock()

	// scendi a ritroso fino ad anchor (index 0)
	for i := len(rt.deBruijn) - 2; i >= 0; i-- {
		rt.dbWinMu[i].RLock()
		candidate := rt.deBruijn[i].Node
		rt.dbWinMu[i].RUnlock()

		if id.InOC(candidate.ID, next.ID) {
			return candidate
		}
		next = candidate
	}
	// fallback: se non trovato, ritorna anchor
	return rt.deBruijn[0].Node
}
