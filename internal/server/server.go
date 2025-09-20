package server

import (
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/node"
	"google.golang.org/grpc"
	"net"
)

type Server struct {
	node       *node.Node // la struttura nodo della dht per gestire le richieste
	grpcServer *grpc.Server
}

func New(node *node.Node) *Server {
	s := &Server{
		grpcServer: grpc.NewServer(),
		node:       node,
	}
	dhtv1.RegisterDHTServer(s.grpcServer, &Handler{node: node})
	return s
}

func (s *Server) Run(lis net.Listener) error {
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}
