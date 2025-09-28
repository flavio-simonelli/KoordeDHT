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

// Server wraps a gRPC server that exposes both the client-facing
// and DHT-internal RPC services.
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	lgr        logger.Logger
}

// New constructs a new Server bound to the given listener and
// associated with the provided Koorde node.
//
// The function registers both the client API and DHT API services
// with the underlying gRPC server. By default, logging is disabled
// (NopLogger) unless overridden via functional options.
//
// Parameters:
//   - lis: network listener to bind the gRPC server to (must be non-nil)
//   - n: Koorde node providing the service logic (must be non-nil)
//   - grpcOpts: optional gRPC server options (e.g., interceptors, TLS)
//   - srvOpts: functional options for configuring the Server itself
//
// Returns:
//   - A pointer to the initialized Server
//   - An error if required arguments are missing
func New(lis net.Listener, n *node.Node, grpcOpts []grpc.ServerOption, srvOpts ...Option) (*Server, error) {
	if lis == nil {
		return nil, fmt.Errorf("server: listener must not be nil")
	}
	if n == nil {
		return nil, fmt.Errorf("server: node must not be nil")
	}

	s := &Server{
		grpcServer: grpc.NewServer(grpcOpts...),
		listener:   lis,
		lgr:        &logger.NopLogger{}, // default: no logging
	}

	// Apply functional options (e.g., custom logger)
	for _, opt := range srvOpts {
		opt(s)
	}

	// Register gRPC services bound to the provided node
	clientv1.RegisterClientAPIServer(s.grpcServer, NewClientService(n))
	dhtv1.RegisterDHTServer(s.grpcServer, NewDHTService(n))

	return s, nil
}

// Start launches the gRPC server and blocks until it is stopped.
// This method should typically be invoked in its own goroutine
// if the caller needs to perform other tasks concurrently.
//
// Returns:
//   - An error if the underlying gRPC server fails to start or
//     terminates unexpectedly.
func (s *Server) Start() error {
	if err := s.grpcServer.Serve(s.listener); err != nil {
		return fmt.Errorf("gRPC server stopped: %w", err)
	}
	return nil
}

// Stop forcefully terminates the gRPC server,
// immediately closing all active connections and
// canceling in-flight RPCs.
//
// This method should be used only for fast shutdowns
// (e.g., during process termination).
func (s *Server) Stop() {
	s.grpcServer.Stop()
}

// GracefulStop attempts to shut down the gRPC server gracefully.
// It stops accepting new connections and RPCs, while waiting for
// all in-flight requests to complete before shutting down.
//
// This is the recommended way to stop the server during normal
// operation, as it avoids dropping client requests.
func (s *Server) GracefulStop() {
	s.grpcServer.GracefulStop()
}
