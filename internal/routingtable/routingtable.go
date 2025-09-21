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
	logger      logger.Logger   // logger per la routing table (default: NopLogger)
	graphGrade  int             // grado del grafo De Bruijn
	self        *routingEntry   // il nodo locale
	succMu      sync.RWMutex    // mutex per il successore
	successor   *routingEntry   // successore nel ring
	predMu      sync.RWMutex    // mutex per il predecessore
	predecessor *routingEntry   // predecessore nel ring
	dbMu        []sync.RWMutex  // mutex per i link De Bruijn
	deBruijn    []*routingEntry // collegamenti De Bruijn (dimensione k)
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
func New(self domain.Node, graphGrade int, opts ...Option) (*RoutingTable, error) {
	if graphGrade < 2 {
		return nil, InvalidDegree
	}
	rt := &RoutingTable{
		self:        &routingEntry{Node: self},
		successor:   &routingEntry{Node: self},
		predecessor: &routingEntry{Node: self},
		deBruijn:    make([]*routingEntry, graphGrade),
		dbMu:        make([]sync.RWMutex, graphGrade),
		logger:      &logger.NopLogger{}, // default: nessun log
	}
	// inizializza i link De Bruijn con il nodo locale
	for i := 0; i < graphGrade; i++ {
		rt.deBruijn[i] = &routingEntry{Node: self}
	}
	// graphGrade
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

// Self restituisce il nodo locale.
func (rt *RoutingTable) Self() domain.Node {
	return rt.self.Node
}

// Successor restituisce il successore del nodo locale.
func (rt *RoutingTable) Successor() domain.Node {
	rt.succMu.RLock()
	n := rt.successor.Node
	rt.succMu.RUnlock()
	return n
}

// SetSuccessor aggiorna il successore del nodo locale.
func (rt *RoutingTable) SetSuccessor(n domain.Node) {
	rt.succMu.Lock()
	old := rt.successor.Node
	rt.successor = &routingEntry{Node: n}
	rt.succMu.Unlock()
	rt.logger.Info("routingtable.SetSuccessor",
		logger.F("old.addr", old.Addr),
		logger.F("new.addr", n.Addr),
		logger.F("old.id", old.ID.ToHexString()),
		logger.F("new.id", n.ID.ToHexString()),
	)
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
	rt.logger.Info("routingtable.SetPredecessor",
		logger.F("old.addr", old.Addr),
		logger.F("new.addr", n.Addr),
		logger.F("old.id", old.ID.ToHexString()),
		logger.F("new.id", n.ID.ToHexString()),
	)
}

// DeBruijn restituisce il nodo De Bruijn all'indice specificato.
func (rt *RoutingTable) DeBruijn(i int) domain.Node {
	if i < 0 || i >= len(rt.deBruijn) {
		return domain.Node{}
	}
	rt.dbMu[i].RLock()
	n := rt.deBruijn[i].Node
	rt.dbMu[i].RUnlock()
	return n
}

// FixDeBruijn aggiorna il nodo De Bruijn all'indice specificato.
func (rt *RoutingTable) FixDeBruijn(i int, n domain.Node) {
	if i < 0 || i >= len(rt.deBruijn) {
		return
	}
	rt.dbMu[i].Lock()
	old := rt.deBruijn[i].Node
	rt.deBruijn[i] = &routingEntry{Node: n}
	rt.dbMu[i].Unlock()
	rt.logger.Info("routingtable.FixDeBruijn",
		logger.F("index", i),
		logger.F("old.addr", old.Addr),
		logger.F("new.addr", n.Addr),
		logger.F("old.id", old.ID.ToHexString()),
		logger.F("new.id", n.ID.ToHexString()),
	)
}

// FindSuccessor cerca il successore di id a partire dal nodo rt.
// Restituisce il nodo successore più vicino a id. Se id è compreso tra rt e il suo successore, allora restituisce il successore e true indicando che è il successore vero.
// Altrimenti restituisce il nodo più vicino a id tra i nodi conosciuti e false.
func (rt *RoutingTable) FindSuccessor(id domain.ID) (domain.Node, bool) {
	rt.succMu.RLock()
	succ := rt.successor.Node
	rt.succMu.RUnlock()
	if id.InOC(rt.self.ID, succ.ID) {
		return succ, true
	}
	// cerca il nodo più vicino a id tra i nodi conosciuti
	// controlla il successore
	// TODO: implementare sia i più link brujin che i più successori
	rt.dbMu[0].RLock()
	closest := rt.deBruijn[0].Node
	rt.dbMu[0].RUnlock()
	if closest.ID.Equal(rt.self.ID) {
		closest = succ
	}
	return closest, false
}
