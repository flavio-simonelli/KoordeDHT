package server

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/logger"
	"KoordeDHT/internal/node"
	"fmt"
	"net"

	"google.golang.org/grpc"
)

// Server wraps a gRPC server hosting both the client and DHT services.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	lgr        logger.Logger
}

// New creates a new gRPC server bound to the given address
// and registers both Client and DHT services.
// You can pass both grpc.ServerOptions and custom server.Options.
func New(lis net.Listener, n *node.Node, grpcOpts []grpc.ServerOption, srvOpts ...Option) (*Server, error) {
	s := &Server{
		grpcServer: grpc.NewServer(grpcOpts...),
		listener:   lis,
		lgr:        &logger.NopLogger{}, // default: no logging
	}
	// Apply functional options (logger)
	for _, opt := range srvOpts {
		opt(s)
	}
	// Register services with a reference to node
	clientv1.RegisterClientAPIServer(s.grpcServer, NewClientService(n))
	dhtv1.RegisterDHTServer(s.grpcServer, NewDHTService(n))
	return s, nil
}

// Start runs the gRPC server and blocks until it stops.
// It returns any error from grpc.Server.Serve.
func (s *Server) Start() error {
	if err := s.grpcServer.Serve(s.listener); err != nil {
		return fmt.Errorf("gRPC server stopped: %w", err)
	}
	return nil
}

// Stop immediately stops the server and closes all active connections.
func (s *Server) Stop() {
	s.grpcServer.Stop()
}

// GracefulStop gracefully shuts down the server,
// waiting for in-flight RPCs to complete.
func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}
