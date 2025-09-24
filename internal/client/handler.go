package client

import (
	pb "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	ErrClientNotInPool = errors.New("clientpool: client not found in pool")
	ErrNoPredecessor   = errors.New("client: remote node has no predecessor")
	ErrTimeout         = errors.New("client: RPC timed out, no response from remote node")
)

// FindSuccessorStart performs the initial FindSuccessor RPC call on the given server.
// It starts a lookup for the provided target ID by sending a request in "Initial" mode.
// The method retrieves a client connection from the pool, builds the request, executes
// the RPC with a timeout, and converts the response into a domain.Node.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStart(target domain.ID, serverAddr string) (*domain.Node, error) {
	// Retrieve the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("FindSuccessorStart: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Build the request in "Initial" mode (first hop of the lookup)
	req := &pb.FindSuccessorRequest{
		TargetId: target,
		Mode: &pb.FindSuccessorRequest_Initial{
			Initial: &pb.Initial{},
		},
	}
	// Perform the RPC
	resp, err := client.FindSuccessor(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("client: FindSuccessor RPC to %s failed: %w", serverAddr, err)
	}
	// Convert the protobuf Node into a domain.Node
	return domain.NodeFromProto(resp.Node), nil
}

// FindSuccessorStep performs a FindSuccessor RPC in "Step" mode.
// It continues a lookup for the given target ID, providing the current
// imaginary node (currentI) and the shifted key state (kshift) as required
// by the Koorde de Bruijn routing algorithm.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStep(target, currentI, kshift domain.ID, serverAddr string) (*domain.Node, error) {
	// Retrieve the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("FindSuccessorStep: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Build the request in "Step" mode (subsequent hop of the lookup)
	req := &pb.FindSuccessorRequest{
		TargetId: target,
		Mode: &pb.FindSuccessorRequest_Step{
			Step: &pb.Step{
				CurrentI: currentI,
				KShift:   kshift,
			},
		},
	}
	// Perform the RPC
	resp, err := client.FindSuccessor(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("client: FindSuccessorStep RPC to %s failed: %w", serverAddr, err)
	}
	// Convert the protobuf Node into a domain.Node
	return domain.NodeFromProto(resp.Node), nil
}

// GetPredecessor contacts the given remote node and asks for its predecessor.
// It uses the default timeout configured in the pool.
//
// Returns:
//   - *domain.Node: the predecessor of the remote node
//   - error: ErrClientNotInPool if the client is not in the pool,
//     ErrTimeout if the RPC timed out,
//     ErrNoPredecessor if the remote node has no predecessor,
//     or a wrapped RPC error otherwise.
func (p *Pool) GetPredecessor(serverAddr string) (*domain.Node, error) {
	// Retrieve the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("GetPredecessor: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}

	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	// Perform the RPC
	resp, err := client.GetPredecessor(ctx, &emptypb.Empty{})
	if err != nil {
		// Timeout
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		// NotFound = no predecessor
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {
			return nil, ErrNoPredecessor
		}
		// Other RPC errors
		return nil, fmt.Errorf("client: GetPredecessor RPC to %s failed: %w", serverAddr, err)
	}

	// Convert proto.Node = domain.Node
	return domain.NodeFromProto(resp), nil
}

// GetSuccessorList contacts the given remote node and retrieves its successor list.
// It uses the default timeout configured in the pool.
//
// Returns:
//   - []*domain.Node: the list of successors returned by the remote node
//   - error: ErrClientNotInPool if the client is not in the pool,
//     ErrTimeout if the RPC timed out,
//     or a wrapped RPC error otherwise.
func (p *Pool) GetSuccessorList(serverAddr string) ([]*domain.Node, error) {
	// Retrieve the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("GetSuccessorList: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Perform the RPC
	resp, err := client.GetSuccessorList(ctx, &emptypb.Empty{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("client: GetSuccessorList RPC to %s failed: %w", serverAddr, err)
	}
	// Convert proto.Node slice = domain.Node slice
	nodes := make([]*domain.Node, len(resp.Successors))
	for i, n := range resp.Successors {
		nodes[i] = domain.NodeFromProto(n)
	}
	return nodes, nil
}

// Notify sends a notification RPC to the given remote node, informing it that
// this node (self) might be its predecessor. This is part of the Chord/Koorde
// stabilization protocol.
// It uses the default timeout configured in the pool.
//
// Returns:
//   - nil on success
//   - ErrClientNotInPool if the client is not in the pool
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func (p *Pool) Notify(self *domain.Node, serverAddr string) error {
	// Retrieve the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("Notify: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Build the request from the domain.Node
	req := self.ToProto()
	// Perform the RPC
	_, err = client.Notify(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Notify RPC to %s failed: %w", serverAddr, err)
	}
	return nil
}

// Ping sends a Ping RPC to the given remote node to check if it is alive.
// It uses the default timeout configured in the pool.
//
// Returns:
//   - nil on success
//   - ErrClientNotInPool if the client is not in the pool
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func (p *Pool) Ping(serverAddr string) error {
	// Retrieve the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("Ping: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Perform the RPC
	_, err = client.Ping(ctx, &emptypb.Empty{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Ping RPC to %s failed: %w", serverAddr, err)
	}
	return nil
}
