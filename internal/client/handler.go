package client

import (
	pb "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/ctxutil"
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
// Using the provided context. This allows propagating cancellation and deadlines across multiple hops in a lookup.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStart(ctx context.Context, target domain.ID, serverAddr string) (*domain.Node, error) {
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return nil, fmt.Errorf("FindSuccessorStart: cannot contact self (%s)", p.selfAddr)
		}
	*/
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
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
	return domain.NodeFromProtoDHT(resp.Node), nil
}

// FindSuccessorStep performs a FindSuccessor RPC in "Step" mode.
// It continues a lookup for the given target ID, providing the current
// imaginary node (currentI) and the shifted key state (kshift) as required
// by the Koorde de Bruijn routing algorithm.
//
// Using the provided context. This allows propagating cancellation and deadlines across multiple hops in a lookup.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrClientNotInPool, ErrTimeout, or a wrapped RPC error
func (p *Pool) FindSuccessorStep(ctx context.Context, target, currentI, kshift domain.ID, serverAddr string) (*domain.Node, error) {
	// check for self-address
	/*
		if serverAddr == p.selfAddr {
			// if contacting self return error
			return nil, fmt.Errorf("FindSuccessorStep: cannot contact self (%s)", p.selfAddr)
		}

	*/
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
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
	return domain.NodeFromProtoDHT(resp.Node), nil
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
func (p *Pool) GetPredecessor(ctx context.Context, serverAddr string) (*domain.Node, error) {
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
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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
	return domain.NodeFromProtoDHT(resp), nil
}

// GetSuccessorList contacts the given remote node and retrieves its successor list.
// It uses the default timeout configured in the pool.
//
// Returns:
//   - []*domain.Node: the list of successors returned by the remote node
//   - error: ErrClientNotInPool if the client is not in the pool,
//     ErrTimeout if the RPC timed out,
//     or a wrapped RPC error otherwise.
func (p *Pool) GetSuccessorList(ctx context.Context, serverAddr string) ([]*domain.Node, error) {
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
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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
		nodes[i] = domain.NodeFromProtoDHT(n)
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
func (p *Pool) Notify(ctx context.Context, self *domain.Node, serverAddr string) error {
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
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Build the request from the domain.Node
	req := self.ToProtoDHT()
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
func (p *Pool) Ping(ctx context.Context, serverAddr string) error {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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

// StoreRemoteWithContext streams a batch of resources to a remote node via the Store RPC.
// It opens a client-stream, sends each resource in the slice, and finally closes the stream.
func (p *Pool) StoreRemoteWithContext(ctx context.Context, resources []domain.Resource, serverAddr string) error {
	client, err := p.Get(serverAddr)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}

	// open the client stream
	stream, err := client.Store(ctx)
	if err != nil {
		return fmt.Errorf("failed to open store stream to %s: %w", serverAddr, err)
	}

	// send each resource
	for _, res := range resources {
		req := &pb.StoreRequest{
			Key:    res.Key,
			RawKey: res.RawKey,
			Value:  res.Value,
		}
		if err := stream.Send(req); err != nil {
			return fmt.Errorf("failed to send resource (rawKey=%s) to %s: %w", res.RawKey, serverAddr, err)
		}
	}

	// close and wait for server ack
	_, err = stream.CloseAndRecv()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("store stream to %s failed: %w", serverAddr, err)
	}

	return nil
}

// RetrieveRemoteWithContext sends a RetrieveValue RPC to the given remote node to fetch
// a resource by its key. It returns the resource if found.
//
// Returns:
//   - *domain.Resource: the resource retrieved from the remote node
//   - error: ErrClientNotInPool if the client is not in the pool,
//     ErrTimeout if the RPC timed out,
//     or a wrapped RPC error otherwise.
func (p *Pool) RetrieveRemoteWithContext(ctx context.Context, key domain.ID, serverAddr string) (*domain.Resource, error) {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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
		Key:    key,
		RawKey: resp.RawKey,
		Value:  resp.Value,
	}
	return res, nil
}

// RemoveRemoteWithContext sends a RemoveValue RPC to the given remote node to delete
// a resource by its key.
//
// Returns:
//   - nil on success
//   - ErrClientNotInPool if the client is not in the pool
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func (p *Pool) RemoveRemoteWithContext(ctx context.Context, key domain.ID, serverAddr string) error {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		p.lgr.Warn("Remove: unable to get client from pool",
			logger.F("addr", serverAddr), logger.F("err", err))
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
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

// Leave sends a Leave RPC to the given remote node to inform it that this node is leaving the DHT.
func (p *Pool) Leave(ctx context.Context, self *domain.Node, serverAddr string) error {
	// RetrieveLocal the client from the pool
	client, err := p.Get(serverAddr)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrClientNotInPool, serverAddr)
	}
	// Build the request from the domain.Node
	req := self.ToProtoDHT()
	// Perform the RPC
	_, err = client.Leave(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Leave RPC to %s failed: %w", serverAddr, err)
	}
	return nil
}
