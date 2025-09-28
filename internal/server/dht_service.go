package server

import (
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/ctxutil"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/node"
	"context"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// dhtService implements the DHT gRPC service defined in dht.proto.
// It provides the RPC handlers that allow Koorde nodes to communicate
// with each other for lookups, stabilization, and resource management.
type dhtService struct {
	dhtv1.UnimplementedDHTServer
	node *node.Node
}

// NewDHTService constructs a new DHT gRPC service bound to the given node.
//
// Parameters:
//   - n: pointer to the Koorde node instance providing the logic (must be non-nil)
//
// Returns:
//   - A dhtv1.DHTServer implementation suitable for gRPC registration
//
// Panics if the provided node is nil.
func NewDHTService(n *node.Node) dhtv1.DHTServer {
	if n == nil {
		panic(errors.New("NewDHTService: node must not be nil"))
	}
	return &dhtService{node: n}
}

// TODO: capire dove mettere questa funzione e cosa fare con ste metriche
func idAttributes(prefix string, id domain.ID) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(prefix+".dec", id.ToBigInt().String()),
		attribute.String(prefix+".hex", id.ToHexString(true)),
		attribute.String(prefix+".bin", id.ToBinaryString(true)), // se String() è binario
	}
}

// FindSuccessor handles a request to locate the successor of a given target ID.
// Depending on the mode, the request is treated as either:
//   - Initial: the first hop of a lookup
//   - Step: a subsequent hop with additional state (current imaginary node, shifted key)
//
// Errors:
//   - codes.InvalidArgument: request is malformed or missing fields
//   - codes.NotFound: no successor could be determined
//   - codes.Internal: underlying node logic failed
func (s *dhtService) FindSuccessor(ctx context.Context, req *dhtv1.FindSuccessorRequest) (*dhtv1.FindSuccessorResponse, error) {
	// Validate request
	if req == nil || len(req.TargetId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing target_id")
	}

	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Enrich tracing span (if present)
	if span := trace.SpanFromContext(ctx); span != nil {
		switch mode := req.Mode.(type) {
		case *dhtv1.FindSuccessorRequest_Initial:
			target := domain.ID(req.TargetId)
			span.SetAttributes(
				attribute.String("dht.findsucc.mode", "init"),
			)
			span.SetAttributes(idAttributes("dht.findsucc.target", target)...)

		case *dhtv1.FindSuccessorRequest_Step:
			target := domain.ID(req.TargetId)
			currentI := domain.ID(mode.Step.CurrentI)
			kshift := domain.ID(mode.Step.KShift)

			span.SetAttributes(attribute.String("dht.findsucc.mode", "step"))
			span.SetAttributes(idAttributes("dht.findsucc.target", target)...)
			span.SetAttributes(idAttributes("dht.findsucc.currentI", currentI)...)
			span.SetAttributes(idAttributes("dht.findsucc.kshift", kshift)...)

		default:
			span.SetAttributes(attribute.String("dht.findsucc.mode", "invalid"))
		}
	}

	// validate target ID
	if err := s.node.IsValidID(req.TargetId); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid target_id")
	}
	target := domain.ID(req.TargetId)

	// Dispatch to the appropriate node method
	var (
		succ *domain.Node
		err  error
	)
	switch mode := req.Mode.(type) {
	case *dhtv1.FindSuccessorRequest_Initial:
		// Call FindSuccessorInit for initial lookups
		succ, err = s.node.FindSuccessorInit(ctx, target)
	case *dhtv1.FindSuccessorRequest_Step:
		if err := s.node.IsValidID(mode.Step.CurrentI); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid current_i")
		}
		if err := s.node.IsValidID(mode.Step.KShift); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid kshift")
		}
		currentI := domain.ID(mode.Step.CurrentI)
		kshift := domain.ID(mode.Step.KShift)
		// Call FindSuccessorStep with extracted parameters
		succ, err = s.node.FindSuccessorStep(ctx, target, currentI, kshift)
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid mode")
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "FindSuccessor failed: %v", err)
	}

	if succ == nil {
		return nil, status.Error(codes.NotFound, "successor not found")
	}

	return &dhtv1.FindSuccessorResponse{Node: succ.ToProtoDHT()}, nil
}

// GetPredecessor handles a request to retrieve the current predecessor of this node.
//
// Behavior:
//   - If the context is canceled or its deadline has expired, the request
//     is aborted with the corresponding gRPC status (Canceled/DeadlineExceeded).
//   - If the node has no known predecessor, a NotFound status is returned.
//   - Otherwise, the current predecessor is returned as a protobuf Node.
func (s *dhtService) GetPredecessor(ctx context.Context, _ *emptypb.Empty) (*dhtv1.Node, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Retrieve current predecessor
	pred := s.node.Predecessor()
	if pred == nil {
		return nil, status.Error(codes.NotFound, "no predecessor set")
	}

	return pred.ToProtoDHT(), nil
}

// GetSuccessorList handles a request to retrieve the current successor list of this node.
//
// Behavior:
//   - If the context is canceled or its deadline has expired, the request
//     is aborted with the corresponding gRPC status (Canceled/DeadlineExceeded).
//   - If the successor list is not yet initialized, an empty list is returned.
//   - Otherwise, the full successor list is returned as a protobuf message.
func (s *dhtService) GetSuccessorList(ctx context.Context, _ *emptypb.Empty) (*dhtv1.SuccessorList, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Retrieve current successor list
	succList := s.node.SuccessorList()
	if succList == nil {
		return &dhtv1.SuccessorList{Successors: []*dhtv1.Node{}}, nil
	}

	// Convert domain.Node slice to proto.Node slice
	protoList := make([]*dhtv1.Node, 0, len(succList))
	for _, n := range succList {
		if n != nil { // defensive check: avoid nil pointer panic
			protoList = append(protoList, n.ToProtoDHT())
		}
	}

	return &dhtv1.SuccessorList{Successors: protoList}, nil
}

// Notify handles a stabilization notification from another node,
// indicating that it might be our predecessor.
//
// This RPC is part of the Chord/Koorde stabilization protocol: when
// a node learns of a potential predecessor, it calls Notify on that
// node so the callee can update its predecessor pointer if appropriate.
//
// Behavior:
//   - If the context is canceled or its deadline has expired, the request is aborted
//     with the corresponding gRPC status.
//   - If the request is invalid (missing ID or address, or ID outside the space),
//     an InvalidArgument status is returned.
//   - Otherwise, the node logic is invoked to update the predecessor.
func (s *dhtService) Notify(ctx context.Context, req *dhtv1.Node) (*emptypb.Empty, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request structure
	if req == nil || len(req.Id) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid node: missing ID or address")
	}

	// Convert proto.Node to domain.Node
	n, err := domain.NodeFromProtoDHT(s.node.Space(), req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("invalid node: %v", err))
	}
	// update predecessor
	s.node.Notify(n)

	return &emptypb.Empty{}, nil
}

// Ping is a lightweight liveness check used by other nodes to verify that this node
// is still alive and reachable. It is commonly used in stabilization and failure
// detection routines.
//
// Behavior:
//   - If the context is canceled or the deadline is exceeded, the request is aborted
//     with the corresponding gRPC status.
//   - Otherwise, the method always returns an empty response to indicate liveness.
func (s *dhtService) Ping(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	// Always succeed if node is alive
	return &emptypb.Empty{}, nil
}

// Store handles a client-streaming request to store multiple resources.
// The client sends a stream of StoreRequest messages, and the server replies
// with an Empty once all resources have been processed.
//
// Errors:
//   - codes.InvalidArgument if a request is malformed
//   - codes.Internal if receiving from the stream fails or storing fails
func (s *dhtService) Store(stream dhtv1.DHT_StoreServer) error {
	ctx := stream.Context()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			// client has finished sending requests
			return stream.SendAndClose(&emptypb.Empty{})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive request: %v", err)
		}

		// Validate context
		if cerr := ctxutil.CheckContext(ctx); cerr != nil {
			return cerr
		}

		// Extract and validate resource
		resProto := req.GetResource()
		if resProto == nil {
			return status.Error(codes.InvalidArgument, "missing resource")
		}
		res, convErr := domain.ResourceFromProtoDHT(s.node.Space(), resProto)
		if convErr != nil {
			return status.Errorf(codes.InvalidArgument, "invalid resource: %v", convErr)
		}

		// Store locally
		if serr := s.node.StoreLocal(ctx, *res); serr != nil {
			return status.Errorf(codes.Internal, "failed to store resource: %v", serr)
		}
	}
}

// Retrieve fetches a resource from the local node's storage by its key.
//
// Errors:
//   - codes.InvalidArgument if the request is malformed or the key is invalid
//   - codes.NotFound if the resource does not exist locally
//   - codes.Internal if the storage backend fails
func (s *dhtService) Retrieve(ctx context.Context, req *dhtv1.RetrieveRequest) (*dhtv1.RetrieveResponse, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	if err := s.node.Space().IsValidID(req.Key); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key")
	}
	id := domain.ID(req.Key)

	// Perform local lookup
	res, err := s.node.RetrieveLocal(id)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "key not found")
		}
		return nil, status.Errorf(codes.Internal, "retrieve failed: %v", err)
	}

	// Convert to proto and wrap in RetrieveResponse
	return &dhtv1.RetrieveResponse{
		Resource: res.ToProtoDHT(),
	}, nil
}

// Remove deletes a resource from the local node's storage by its key.
//
// Errors:
//   - codes.InvalidArgument if the request is malformed or the key is invalid
//   - codes.NotFound if the resource does not exist locally
//   - codes.Internal if the storage backend fails
func (s *dhtService) Remove(ctx context.Context, req *dhtv1.RemoveRequest) (*emptypb.Empty, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	if err := s.node.Space().IsValidID(req.Key); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key")
	}
	id := domain.ID(req.Key)

	// Perform local delete
	if err := s.node.RemoveLocal(id); err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "key not found")
		}
		return nil, status.Errorf(codes.Internal, "remove failed: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// Leave handles a request from a successor node indicating that it is leaving the network.
//
// Behavior:
//   - If the context is canceled or its deadline has expired, the request is aborted.
//   - If the request is invalid (nil or missing ID/address), an InvalidArgument status is returned.
//   - Otherwise, the node logic is invoked to handle the departure of the leaving node.
//
// Errors:
//   - codes.InvalidArgument if the request is malformed
//   - codes.Internal if the node conversion fails or internal handling fails
func (s *dhtService) Leave(
	ctx context.Context,
	req *dhtv1.Node,
) (*emptypb.Empty, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || len(req.Id) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid leaving node")
	}

	// Convert proto.Node to domain.Node
	nodeLeaving, err := domain.NodeFromProtoDHT(s.node.Space(), req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert leaving node: %v", err)
	}

	// Handle node departure
	if herr := s.node.HandleLeave(nodeLeaving); herr != nil {
		return nil, status.Errorf(codes.Internal, "failed to handle leave: %v", herr)
	}

	return &emptypb.Empty{}, nil
}
