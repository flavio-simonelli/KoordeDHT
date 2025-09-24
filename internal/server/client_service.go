package server

import (
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
