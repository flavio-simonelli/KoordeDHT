package server

import (
	"context"

	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/node"
)

// dhtService implements the DHTServiceServer interface.
type dhtService struct {
	dhtv1.UnimplementedDHTServer
	node *node.Node
}

func NewDHTService(n *node.Node) dhtv1.DHTServer {
	return &dhtService{node: n}
}

// Example RPC handler: FindSuccessor
func (s *dhtService) FindSuccessor(ctx context.Context, req *dhtv1.FindSuccessorRequest) (*dhtv1.FindSuccessorResponse, error) {
	// Use s.node to query routing table, etc.
	succ := s.node.FindSuccessor(req.Id)
	return &dhtv1.FindSuccessorResponse{
		Node: succ.ToProto(),
	}, nil
}
