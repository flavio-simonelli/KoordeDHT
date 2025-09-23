package server

import (
	"context"
	"fmt"

	clientv1 "KoordeDHT/internal/api/client/v1"
	"KoordeDHT/internal/node"
)

// clientService implements the ClientServiceServer interface.
type clientService struct {
	clientv1.UnimplementedClientAPIServer
	node *node.Node
}

func NewClientService(n *node.Node) clientv1.ClientAPIServer {
	return &clientService{node: n}
}

// Example RPC handler: GetSuccessorList
func (s *clientService) GetSuccessorList(ctx context.Context, req *clientv1.GetRequest) (*clientv1.GetResponse, error) {
	val, ok := s.node.GetSuccessorList(req.Key)
	if !ok {
		return nil, fmt.Errorf("key not found")
	}
	return &clientv1.GetResponse{Value: val}, nil
}
