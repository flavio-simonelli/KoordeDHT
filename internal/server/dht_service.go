package server

import (
	"KoordeDHT/internal/domain"
	"context"
	"errors"

	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"KoordeDHT/internal/node"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// dhtService implements the DHTServiceServer interface defined in the .proto.
// It provides RPC handlers for node-to-node communication in the Koorde DHT.
type dhtService struct {
	dhtv1.UnimplementedDHTServer
	node *node.Node
}

// NewDHTService creates a new DHT service bound to the given node.
// The node reference is used internally to access routing and state.
func NewDHTService(n *node.Node) dhtv1.DHTServer {
	return &dhtService{node: n}
}

// FindSuccessor handles a request to locate the successor of a given target ID.
// Depending on the mode, the request is treated as either:
//   - Initial: the first hop of a lookup
//   - Step: a subsequent hop with additional state (current imaginary node, shifted key)
//
// Returns NotFound if no successor can be determined, or Internal in case of errors
// inside the node logic.
func (s *dhtService) FindSuccessor(ctx context.Context, req *dhtv1.FindSuccessorRequest) (*dhtv1.FindSuccessorResponse, error) {
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// Validate request
	if req == nil || len(req.TargetId) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing target_id")
	}
	target := domain.ID(req.TargetId)
	var succ *domain.Node
	var err error
	// Select handling depending on request mode
	switch mode := req.Mode.(type) {
	case *dhtv1.FindSuccessorRequest_Initial:
		succ, err = s.node.FindSuccessorInit(ctx, target)
	case *dhtv1.FindSuccessorRequest_Step:
		currentI := domain.ID(mode.Step.CurrentI)
		kshift := domain.ID(mode.Step.KShift)
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
	return &dhtv1.FindSuccessorResponse{Node: succ.ToProto()}, nil
}

// GetPredecessor handles a request to retrieve the current predecessor of this node.
//
// Behavior:
//   - If the context is canceled or deadline exceeded, the request is aborted with a Canceled/DeadlineExceeded status.
//   - If no predecessor is known, a NotFound status is returned.
//   - Otherwise, the current predecessor is returned as a protobuf Node.
func (s *dhtService) GetPredecessor(ctx context.Context, _ *emptypb.Empty) (*dhtv1.Node, error) {
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// RetrieveLocal predecessor from node
	pred := s.node.Predecessor()
	if pred == nil {
		return nil, status.Error(codes.NotFound, "no predecessor set")
	}
	return pred.ToProto(), nil
}

// GetSuccessorList handles a request to retrieve the current successor list of this node.
//
// Behavior:
//   - If the context is canceled or the deadline is exceeded, the request is aborted
//     with a corresponding gRPC status (Canceled or DeadlineExceeded).
//   - If the successor list is nil (not initialized), an empty list is returned.
//   - Otherwise, the current successor list is returned as a protobuf message.
func (s *dhtService) GetSuccessorList(ctx context.Context, _ *emptypb.Empty) (*dhtv1.SuccessorList, error) {
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// RetrieveLocal successor list from node
	succList := s.node.SuccessorList()
	if succList == nil {
		return &dhtv1.SuccessorList{Successors: []*dhtv1.Node{}}, nil
	}
	// Convert domain.Node → proto.Node
	protoList := make([]*dhtv1.Node, len(succList))
	for i, n := range succList {
		protoList[i] = n.ToProto()
	}
	return &dhtv1.SuccessorList{
		Successors: protoList,
	}, nil
}

// Notify handles a notification from another node indicating that it might be
// our predecessor. This is part of the stabilization protocol: when a node
// learns of another node that could be its predecessor, it calls Notify on that node.
//
// Behavior:
//   - If the context is canceled or the deadline is exceeded, the request is aborted.
//   - If the request is invalid (missing ID or address), an InvalidArgument status is returned.
//   - Otherwise, the node logic is invoked to update the predecessor if appropriate.
func (s *dhtService) Notify(ctx context.Context, req *dhtv1.Node) (*emptypb.Empty, error) {
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// Validate request
	if req == nil || len(req.Id) == 0 || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid node")
	}
	// Convert proto.Node → domain.Node and update predecessor
	n := domain.NodeFromProto(req)
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
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	// Always succeed if node is alive
	return &emptypb.Empty{}, nil
}

// Store saves or updates a resource in the local node's storage.
func (s *dhtService) Store(ctx context.Context, req *dhtv1.StoreRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	if len(req.Value) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing value")
	}
	// Validate key
	id := domain.ID(req.Key)
	if err := s.node.CheckIdValidity(id); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key")
	}
	// create domain resource
	res := domain.Resource{
		Key:   id,
		Value: req.Value,
	}
	// call store operation
	err := s.node.StoreLocal(res)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Retrieve fetches a resource from the local node's storage by its key.
func (s *dhtService) Retrieve(ctx context.Context, req *dhtv1.RetrieveRequest) (*dhtv1.RetrieveResponse, error) {
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	// Validate key
	id := domain.ID(req.Key)
	if err := s.node.CheckIdValidity(id); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key")
	}
	// call retrieve operation
	res, err := s.node.RetrieveLocal(id)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "key not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &dhtv1.RetrieveResponse{Value: res.Value}, nil
}

// Remove delete a resource from the local node's storage by its key.
func (s *dhtService) Remove(ctx context.Context, req *dhtv1.RemoveRequest) (*emptypb.Empty, error) {
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	// Validate key
	id := domain.ID(req.Key)
	if err := s.node.CheckIdValidity(id); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid key")
	}
	// call delete operation
	err := s.node.RemoveLocal(id)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "key not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}
