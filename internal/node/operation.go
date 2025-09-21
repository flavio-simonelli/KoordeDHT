package node

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
)

// FindSuccessor implementa il lookup di Koorde.
// target   = chiave finale da risolvere
// currentI = nodo immaginario corrente
// kshift   = chiave shiftata (cifre rimanenti)
func (n *Node) FindSuccessor(target, currentI, kshift domain.ID) (domain.Node, error) {
	// se target è tra (myID, succ] allora succ è il successore di target
	succ := n.rt.Successor()
	if target.InOC(n.rt.Self().ID, succ.ID) {
		return succ, nil
	}
	// controlla che io sia effettivamente il nodo precedente di currentI
	if !currentI.InOC(n.rt.Self().ID, succ.ID) {
		// se non lo sono, contatta il mio successore
		return n.cp.FindSuccessor(target, currentI, kshift, succ.Addr) //TODO: considerare la successor list
	}
	// Determina la prossima cifra di kshift da usare (la più significativa base degreeGraph)
	d, err := kshift.TopDigit(n.rt.Degree())
	if err != nil {
		return domain.Node{}, err
	}
	// Calcola il nodo immaginario successivo nextI = (currentI + d*2^(m/b)) mod 2^m
	nextI := currentI.AdvanceDeBruijn(d, n.rt.Degree())
	// Calcola il prossimo kshift = kshift shiftato a sinistra di una cifra in base degreeGraph
	nextKShift, err := kshift.ShiftLeftDigit(n.rt.Degree())
	if err != nil {
		return domain.Node{}, err
	}
	// Trova il miglior nodo vicino (closest preceding node) per target
	pred := n.rt.FindPredecessor(target)
	if pred.ID.Equal(n.rt.Self().ID) {
		// se il predecessore è me stesso, allora effettua un altra iterazione
		return n.FindSuccessor(target, nextI, nextKShift)
	}
	// altrimenti inoltra la richiesta a pred
	return n.cp.FindSuccessor(target, nextI, nextKShift, pred.Addr)
}

// FindPredecessor funziona allo stesso modo di FindSuccessor ma restituisce il predecessore.
// target   = chiave finale da risolvere
// currentI = nodo immaginario corrente
// kshift   = chiave shiftata (cifre rimanenti)
func (n *Node) FindPredecessor(target, currentI, kshift domain.ID) (domain.Node, error) {
	// se target è tra (myID, succ] allora succ è il successore di target
	succ := n.rt.Successor()
	if target.InOC(n.rt.Self().ID, succ.ID) {
		return n.rt.Self(), nil
	}
	// Determina la prossima cifra di kshift da usare (la più significativa base degreeGraph)
	d, err := kshift.TopDigit(n.rt.Degree())
	if err != nil {
		return domain.Node{}, err
	}
	// Calcola il nodo immaginario successivo nextI = (currentI + d*2^(m/b)) mod 2^m
	nextI := currentI.AdvanceDeBruijn(d, n.rt.Degree())
	// Calcola il prossimo kshift = kshift shiftato a sinistra di una cifra in base degreeGraph
	nextKShift, err := kshift.ShiftLeftDigit(n.rt.Degree())
	if err != nil {
		return domain.Node{}, err
	}
	// Trova il miglior nodo vicino (closest preceding node) per target
	pred := n.rt.FindPredecessor(target)
	if pred.ID.Equal(n.rt.Self().ID) {
		// se il predecessore è me stesso, allora effettua un altra iterazione
		return n.FindPredecessor(target, nextI, nextKShift)
	}
	// altrimenti inoltra la richiesta a pred
	return n.cp.FindPredecessor(target, nextI, nextKShift, pred.Addr)
}

func (n *Node) GetPredecessor() domain.Node {
	pred := n.rt.Predecessor()
	n.lgr.Info("GetPredecessor", logger.FNode("predecessor", pred))
	return pred
}

func (n *Node) GetSuccessor() domain.Node {
	succ := n.rt.Successor()
	n.lgr.Info("GetSuccessor", logger.FNode("successor", succ))
	return succ
}

func (n *Node) Notify(m domain.Node) {
	self := n.rt.Self()
	pred := n.rt.Predecessor()
	// se non ho predecessore, o m è tra (pred, self) → aggiorno
	if pred.ID.Equal(self.ID) || m.ID.InOO(pred.ID, self.ID) {
		n.lgr.Info("Notify: updating predecessor",
			logger.FNode("old_predecessor", pred),
			logger.FNode("new_predecessor", m),
		)
		n.rt.SetPredecessor(m)
	} else {
		// altrimenti ignoro
		n.lgr.Debug("Notify: ignored candidate predecessor",
			logger.FNode("current_predecessor", pred),
			logger.FNode("candidate", m),
		)
	}
}

func (n *Node) Put(r domain.Resource) error {
	err := n.s.Put(r)
	if err != nil {
		return err
	}
	n.lgr.Info("Put: resource stored", logger.FResource("resource", r))
	return nil
}

func (n *Node) Get(key domain.ID) (domain.Resource, error) {
	res, err := n.s.Get(key)
	if err != nil {
		return domain.Resource{}, err
	}
	// TODO: qui dobbiamo inviare la richiesta di get al nodo che ha la risorsa se io non ce l'ho localmente
	n.lgr.Info("Get: resource retrieved", logger.FResource("resource", res))
	return res, nil
}

func (n *Node) Delete(key domain.ID) error {
	err := n.s.Delete(key)
	// TODO: qui dobbiamo inviare la richiesta di delete al nodo che ha la risorsa se io non sono il responsabile
	if err != nil {
		return err
	}
	n.lgr.Info("Delete: resource deleted", logger.F("key", key.ToHexString()))
	return nil
}
