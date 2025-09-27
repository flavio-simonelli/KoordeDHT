package domain

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	dhtv1 "KoordeDHT/internal/api/dht/v1"
)

// Node rappresenta un partecipante alla DHT
type Node struct {
	ID   ID     // identificatore nello spazio 2^b
	Addr string // indirizzo di rete, es. "127.0.0.1:5000"
}

// ToProtoDHT converts a domain.Node into its protobuf representation.
func (n *Node) ToProtoDHT() *dhtv1.Node {
	if n == nil {
		return nil
	}
	return &dhtv1.Node{
		Id:      n.ID,
		Address: n.Addr,
	}
}

// NodeFromProtoDHT converts a protobuf Node into a domain.Node.
func NodeFromProtoDHT(p *dhtv1.Node) *Node {
	if p == nil {
		return nil
	}
	return &Node{
		ID:   p.Id,
		Addr: p.Address,
	}
}

// ToProtoClient converts a domain.Node into its protobuf representation.
func (n *Node) ToProtoClient() *clientv1.NodeInfo {
	if n == nil {
		return nil
	}
	return &clientv1.NodeInfo{
		Id:   n.ID.String(),
		Addr: n.Addr,
	}
}
