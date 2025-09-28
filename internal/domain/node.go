package domain

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"fmt"
)

// Node represents a participant in the Koorde DHT.
// Each node has a unique identifier (ID) in the identifier space [0, 2^Bits-1]
// and a network address (host:port).
type Node struct {
	ID   ID     // Identifier within the DHT space
	Addr string // Network address, e.g. "127.0.0.1:5000"
}

// ToProtoDHT converts a domain.Node into its DHT service
// protobuf representation (dht.v1.Node).
//
// Returns nil if the receiver is nil.
func (n *Node) ToProtoDHT() *dhtv1.Node {
	if n == nil {
		return nil
	}
	return &dhtv1.Node{
		Id:      n.ID,
		Address: n.Addr,
	}
}

// NodeFromProtoDHT converts a protobuf DHT node (dht.v1.Node)
// into its domain representation.
//
// Returns nil if the input is nil, or an error if the ID is invalid.
func NodeFromProtoDHT(sp Space, p *dhtv1.Node) (*Node, error) {
	if p == nil {
		return nil, nil
	}
	if err := sp.IsValidID(p.Id); err != nil {
		return nil, fmt.Errorf("invalid DHT node ID: %w", err)
	}
	return &Node{
		ID:   p.Id,
		Addr: p.Address,
	}, nil
}

// ToProtoClient converts a domain.Node into its client-facing
// protobuf representation (client.v1.NodeInfo).
//
// Returns nil if the receiver is nil.
func (n *Node) ToProtoClient() *clientv1.NodeInfo {
	if n == nil {
		return nil
	}
	return &clientv1.NodeInfo{
		Id:   n.ID.ToHexString(true), // Client API expects string ID, not raw bytes
		Addr: n.Addr,
	}
}

// NodeFromProtoClient converts a client-facing protobuf node (client.v1.NodeInfo)
// into its domain representation, decoding the string ID into raw bytes.
//
// The ID string is expected to be in hexadecimal form, optionally prefixed with "0x".
// Returns nil if the input is nil, or an error if the ID is invalid.
func NodeFromProtoClient(sp Space, p *clientv1.NodeInfo) (*Node, error) {
	if p == nil {
		return nil, nil
	}
	id, err := sp.FromHexString(p.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to parse client node ID %q: %w", p.Id, err)
	}
	if err := sp.IsValidID(id); err != nil {
		return nil, fmt.Errorf("invalid client node ID: %w", err)
	}
	return &Node{
		ID:   id,
		Addr: p.Addr,
	}, nil
}
