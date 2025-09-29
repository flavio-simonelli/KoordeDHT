package client

import (
	pb "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/ctxutil"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/telemetry"
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	ErrNoPredecessor = errors.New("client: remote node has no predecessor")
	ErrTimeout       = errors.New("client: RPC timed out, no response from remote node")
)

// FindSuccessorStart performs the initial FindSuccessor RPC call.
// It starts a lookup for the provided target ID by sending a request in "Initial" mode.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrTimeout if the RPC timed out, or a wrapped RPC error otherwise.
func FindSuccessorStart(ctx context.Context, client pb.DHTClient, sp *domain.Space, target domain.ID) (*domain.Node, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
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
		return nil, fmt.Errorf("client: FindSuccessorStart RPC failed: %w", err)
	}

	// Convert the protobuf Node into a domain.Node
	return domain.NodeFromProtoDHT(sp, resp.Node)
}

// FindSuccessorStep performs a FindSuccessor RPC in "Step" mode.
// It continues a lookup for the given target ID, providing the current
// imaginary node (currentI) and the shifted key state (kshift) as required
// by the Koorde de Bruijn routing algorithm.
//
// The caller is responsible for providing a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - *domain.Node: the successor node returned by the remote server
//   - error: ErrTimeout if the RPC timed out, or a wrapped RPC error otherwise.
func FindSuccessorStep(ctx context.Context, client pb.DHTClient, sp *domain.Space, target, currentI, kshift domain.ID) (*domain.Node, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	// Enrich tracing span (if present)
	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(attribute.String("dht.findsucc.mode", "step"))
		span.SetAttributes(telemetry.IdAttributes("dht.findsucc.target", target)...)
		span.SetAttributes(telemetry.IdAttributes("dht.findsucc.currentI", currentI)...)
		span.SetAttributes(telemetry.IdAttributes("dht.findsucc.kshift", kshift)...)
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
		return nil, fmt.Errorf("client: FindSuccessorStep RPC failed: %w", err)
	}
	// Convert the protobuf Node into a domain.Node
	return domain.NodeFromProtoDHT(sp, resp.Node)
}

// GetPredecessor contacts the given remote node and asks for its predecessor.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - *domain.Node: the predecessor of the remote node
//   - error: ErrTimeout if the RPC timed out,
//     ErrNoPredecessor if the remote node has no predecessor,
//     or a wrapped RPC error otherwise.
func GetPredecessor(ctx context.Context, client pb.DHTClient, sp *domain.Space) (*domain.Node, error) {
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
		return nil, fmt.Errorf("client: GetPredecessor RPC failed: %w", err)
	}

	// Convert proto.Node to domain.Node
	return domain.NodeFromProtoDHT(sp, resp)
}

// GetSuccessorList contacts the given remote node and retrieves its successor list.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - []*domain.Node: the list of successors returned by the remote node
//   - error: ErrTimeout if the RPC timed out,
//     or a wrapped RPC error otherwise.
func GetSuccessorList(ctx context.Context, client pb.DHTClient, sp *domain.Space) ([]*domain.Node, error) {
	// Perform the RPC
	resp, err := client.GetSuccessorList(ctx, &emptypb.Empty{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("client: GetSuccessorList RPC failed: %w", err)
	}

	// Convert proto.Node slice to domain.Node slice
	nodes := make([]*domain.Node, len(resp.Successors))
	for i, n := range resp.Successors {
		dn, err := domain.NodeFromProtoDHT(sp, n)
		if err != nil {
			return nil, fmt.Errorf("client: invalid node in successor list: %w", err)
		}
		nodes[i] = dn
	}
	return nodes, nil
}

// Notify sends a notification RPC to the given remote node, informing it that
// this node (self) might be its predecessor. This is part of the Chord/Koorde
// stabilization protocol.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - nil on success
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func Notify(ctx context.Context, client pb.DHTClient, self *domain.Node) error {
	// Build the request from the domain.Node
	req := self.ToProtoDHT()

	// Perform the RPC
	_, err := client.Notify(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Notify RPC failed: %w", err)
	}
	return nil
}

// Ping sends a Ping RPC to the given remote node to check if it is alive.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - nil on success
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func Ping(ctx context.Context, client pb.DHTClient) error {
	// Perform the RPC
	_, err := client.Ping(ctx, &emptypb.Empty{})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Ping RPC failed: %w", err)
	}
	return nil
}

// StoreRemote streams a batch of resources to a remote node via the Store RPC.
// It opens a client-stream, sends each resource in the slice, and finally closes the stream.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - nil on success
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func StoreRemote(ctx context.Context, client pb.DHTClient, resources []domain.Resource) error {
	// Open the client stream
	stream, err := client.Store(ctx)
	if err != nil {
		return fmt.Errorf("client: failed to open store stream: %w", err)
	}

	// Send each resource
	for _, res := range resources {
		req := &pb.StoreRequest{
			Resource: res.ToProtoDHT(),
		}
		if err := stream.Send(req); err != nil {
			return fmt.Errorf("client: failed to send resource (rawKey=%s): %w", res.RawKey, err)
		}
	}

	// Close and wait for server ack
	_, err = stream.CloseAndRecv()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: store stream failed: %w", err)
	}

	return nil
}

// RetrieveRemote sends a RetrieveValue RPC to the given remote node to fetch
// a resource by its key. It returns the resource if found.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - *domain.Resource: the resource retrieved from the remote node
//   - error: ErrTimeout if the RPC timed out,
//     or a wrapped RPC error otherwise.
func RetrieveRemote(ctx context.Context, client pb.DHTClient, sp *domain.Space, key domain.ID) (*domain.Resource, error) {
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
		return nil, fmt.Errorf("client: Retrieve RPC failed: %w", err)
	}

	// Convert proto to domain.Resource
	res, convErr := domain.ResourceFromProtoDHT(sp, resp.Resource)
	if convErr != nil {
		return nil, fmt.Errorf("client: failed to convert resource: %w", convErr)
	}

	return res, nil
}

// RemoveRemote sends a RemoveValue RPC to the given remote node to delete
// a resource by its key.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - nil on success
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func RemoveRemote(ctx context.Context, client pb.DHTClient, key domain.ID) error {
	// Build the request with the key
	req := &pb.RemoveRequest{
		Key: key,
	}

	// Perform the RPC
	_, err := client.Remove(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Remove RPC failed: %w", err)
	}

	return nil
}

// Leave sends a Leave RPC to the given remote node to inform it that this node is leaving the DHT.
//
// The caller must provide a ready-to-use gRPC client.
// This function does not manage client connection pooling or closing.
//
// Returns:
//   - nil on success
//   - ErrTimeout if the RPC timed out
//   - a wrapped RPC error otherwise
func Leave(ctx context.Context, client pb.DHTClient, self *domain.Node) error {
	// Build the request from the domain.Node
	req := self.ToProtoDHT()

	// Perform the RPC
	_, err := client.Leave(ctx, req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return ErrTimeout
		}
		return fmt.Errorf("client: Leave RPC failed: %w", err)
	}
	return nil
}
