package domain

import dhtv1 "KoordeDHT/internal/api/dht/v1"

// Node rappresenta un partecipante alla DHT
type Node struct {
	ID   ID     // identificatore nello spazio 2^b
	Addr string // indirizzo di rete, es. "127.0.0.1:5000"
}

// ToProto converts a domain.Node into its protobuf representation.
func (n *Node) ToProto() *dhtv1.Node {
	if n == nil {
		return nil
	}
	return &dhtv1.Node{
		Id:      n.ID,
		Address: n.Addr,
	}
}

// NodeFromProto converts a protobuf Node into a domain.Node.
func NodeFromProto(p *dhtv1.Node) *Node {
	if p == nil {
		return nil
	}
	return &Node{
		ID:   p.Id,
		Addr: p.Address,
	}
}
