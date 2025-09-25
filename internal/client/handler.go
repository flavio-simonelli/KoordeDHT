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

// checkContext checks whether the provided context has been canceled
// or has exceeded its deadline. If so, it returns the corresponding
// gRPC status error. Otherwise, it returns nil.
func checkContext(ctx context.Context) error {
	switch err := ctx.Err(); {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request was canceled by client")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "request deadline exceeded")
	default:
		return nil
	}
}

// FindSuccessorStart performs the initial FindSuccessor RPC call on the given server.
// It starts a lookup for the provided target ID by sending a request in "Initial" mode.
// The method retrieves a client connection from the pool, builds the request, executes
// the RPC with a timeout, and converts the response into a domain.Node.
//
// This method uses the default timeout configured in the pool.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStart(target domain.ID, serverAddr string) (*domain.Node, error) {
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	return p.FindSuccessorStartWithContext(ctx, target, serverAddr)
}

//TODO: gestire il caso in cui si contatta se stessi

// FindSuccessorStartWithContext performs the initial FindSuccessor RPC call on the given server.
// It starts a lookup for the provided target ID by sending a request in "Initial" mode.
// The method retrieves a client connection from the pool, builds the request, executes
// the RPC with a timeout, and converts the response into a domain.Node.
//
// Using the provided context. This allows propagating cancellation and deadlines across multiple hops in a lookup.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStartWithContext(ctx context.Context, target domain.ID, serverAddr string) (*domain.Node, error) {
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return nil, fmt.Errorf("FindSuccessorStart: cannot contact self (%s)", p.selfAddr)
		}
	*/
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("FindSuccessorStart: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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

// FindSuccessorStepWithContext performs a FindSuccessor RPC in "Step" mode.
// It continues a lookup for the given target ID, providing the current
// imaginary node (currentI) and the shifted key state (kshift) as required
// by the Koorde de Bruijn routing algorithm.
//
// Using the provided context. This allows propagating cancellation and deadlines across multiple hops in a lookup.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStepWithContext(ctx context.Context, target, currentI, kshift domain.ID, serverAddr string) (*domain.Node, error) {
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return nil, fmt.Errorf("FindSuccessorStep: cannot contact self (%s)", p.selfAddr)
		}

	*/
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("FindSuccessorStep: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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

// FindSuccessorStep performs a FindSuccessor RPC in "Step" mode, creating
// a new context with the default timeout configured in the pool.
// This is used at the first hop when starting a lookup locally.
func (p *Pool) FindSuccessorStep(target, currentI, kshift domain.ID, serverAddr string) (*domain.Node, error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	return p.FindSuccessorStepWithContext(ctx, target, currentI, kshift, serverAddr)
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
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return nil, fmt.Errorf("GetPredecessor: cannot contact self (%s)", p.selfAddr)
		}

	*/
	// RetrieveLocal the client from the pool
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
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return nil, fmt.Errorf("GetSuccessorList: cannot contact self (%s)", p.selfAddr)
		}
	*/
	// RetrieveLocal the client from the pool
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
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return fmt.Errorf("Notify: cannot contact self (%s)", p.selfAddr)
		}
	*/
	// RetrieveLocal the client from the pool
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
	// RetrieveLocal the client from the pool
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

// StoreRemote sends a StoreValue RPC to the given remote node to store
func (p *Pool) StoreRemote(res domain.Resource, serverAddr string) error {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("Store: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Build the request from the domain.Resource
	req := &pb.StoreRequest{
		Key:   res.Key,
		Value: res.Value,
	}
	// Perform the RPC
	_, err = client.Store(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Store RPC to %s failed: %w", serverAddr, err)
	}
	return nil
}

// RetrieveRemote sends a RetrieveValue RPC to the given remote node to fetch
// a resource by its key. It returns the resource if found.
//
// Returns:
//   - *domain.Resource: the resource retrieved from the remote node
//   - error: ErrClientNotInPool if the client is not in the pool,
//     ErrTimeout if the RPC timed out,
//     or a wrapped RPC error otherwise.
func (p *Pool) RetrieveRemote(key domain.ID, serverAddr string) (*domain.Resource, error) {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("Retrieve: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Build the request with the key
	req := &pb.RetrieveRequest{
		Key: key,
	}
	// Perform the RPC
	resp, err := client.Retrieve(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("client: Retrieve RPC to %s failed: %w", serverAddr, err)
	}
	// Build the domain.Resource from the response
	res := &domain.Resource{
		Key:   key,
		Value: resp.Value,
	}
	return res, nil
}

// RemoveRemote sends a RemoveValue RPC to the given remote node to delete
// a resource by its key.
//
// Returns:
//   - nil on success
//   - ErrClientNotInPool if the client is not in the pool
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func (p *Pool) RemoveRemote(key domain.ID, serverAddr string) error {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("Remove: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Context with timeout for the RPC
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	// Build the request with the key
	req := &pb.RemoveRequest{
		Key: key,
	}
	// Perform the RPC
	_, err = client.Remove(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Remove RPC to %s failed: %w", serverAddr, err)
	}
	return nil
}
