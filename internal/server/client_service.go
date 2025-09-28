package server

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	"KoordeDHT/internal/ctxutil"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/node"
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// clientService implements the client-facing gRPC API defined in client.proto.
// It provides RPC handlers for external clients to interact with the DHT,
// such as Put, Get, and Delete operations.
//
// Unlike dhtService (which is used for node-to-node communication),
// clientService is intended for end-user clients.
type clientService struct {
	clientv1.UnimplementedClientAPIServer            // forward compatibility with proto changes
	node                                  *node.Node // reference to the local Koorde node
}

// NewClientService constructs a new client-facing gRPC service bound to the given node.
//
// Parameters:
//   - n: pointer to the local Koorde node instance (must be non-nil)
//
// Returns:
//   - A clientv1.ClientAPIServer implementation suitable for gRPC registration.
//
// Panics if the provided node is nil.
func NewClientService(n *node.Node) clientv1.ClientAPIServer {
	if n == nil {
		panic("NewClientService: node must not be nil")
	}
	return &clientService{node: n}
}

// Put handles a client Put RPC call, storing a resource in the DHT.
//
// Behavior:
//   - If the context is canceled or its deadline expires, the call is aborted.
//   - If the request is invalid (nil resource, missing key/value), an InvalidArgument error is returned.
//   - Otherwise, the resource is converted into a domain.Resource, its ID is computed
//     by hashing the raw key, and it is inserted into the DHT via the local node.
func (s *clientService) Put(ctx context.Context, req *clientv1.PutRequest) (*emptypb.Empty, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || req.Resource == nil {
		return nil, status.Error(codes.InvalidArgument, "missing resource")
	}
	if req.Resource.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	if req.Resource.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "missing value")
	}

	// Convert client resource to domain resource (ID derived from RawKey)
	res := domain.ResourceFromProtoClient(s.node.Space(), req.Resource)

	// Store resource
	if err := s.node.Put(ctx, res); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to store resource: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// Get retrieves a resource by its raw key.
//
// Behavior:
//   - If the context is canceled or its deadline expires, the call is aborted.
//   - If the request is invalid (nil or missing key), an InvalidArgument error is returned.
//   - If the resource does not exist, a NotFound error is returned.
//   - Otherwise, the resource is returned in the response.
func (s *clientService) Get(
	ctx context.Context,
	req *clientv1.GetRequest,
) (*clientv1.GetResponse, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}

	// Derive ID from raw key
	id := s.node.Space().NewIdFromString(req.Key)

	// Lookup resource
	res, err := s.node.Get(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "resource not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to retrieve resource: %v", err)
	}
	if res == nil {
		return nil, status.Error(codes.NotFound, "resource not found")
	}

	// Convert to client-facing response using helper
	return &clientv1.GetResponse{
		Value: res.Value,
	}, nil
}

// Delete removes a resource by its raw key.
//
// Behavior:
//   - If the context is canceled or its deadline expires, the call is aborted.
//   - If the request is invalid (nil or missing key), an InvalidArgument error is returned.
//   - If the resource does not exist, a NotFound error is returned.
//   - Otherwise, the resource is removed from the DHT.
func (s *clientService) Delete(ctx context.Context, req *clientv1.DeleteRequest) (*emptypb.Empty, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}

	// Derive ID from raw key
	id := s.node.Space().NewIdFromString(req.Key)

	// Perform delete
	if err := s.node.Delete(ctx, id); err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "resource not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to delete resource: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// GetStore streams all key-value resources stored on this node to the client.
//
// Behavior:
//   - If the context is canceled or its deadline expires, the stream is aborted.
//   - Each stored resource is streamed as a GetStoreResponse, containing
//     both the raw key (id) and its client-facing Resource representation.
func (s *clientService) GetStore(_ *emptypb.Empty, stream clientv1.ClientAPI_GetStoreServer) error {
	// Validate context
	if err := ctxutil.CheckContext(stream.Context()); err != nil {
		return err
	}
	// Retrieve all local resources
	resources := s.node.GetAllResourceStored()
	for _, r := range resources {

		// Check context for cancellation at each step
		if err := ctxutil.CheckContext(stream.Context()); err != nil {
			return err
		}

		res := &clientv1.GetStoreResponse{
			Id: r.Key.ToHexString(true),
			Item: &clientv1.Resource{
				Key:   r.RawKey,
				Value: r.Value,
			},
		}

		// Send over the stream
		if err := stream.Send(res); err != nil {
			return status.Errorf(codes.Internal, "failed to send resource: %v", err)
		}
	}
	return nil
}

// GetRoutingTable returns the current routing table of the node.
//
// Behavior:
//   - If the context is canceled or its deadline expires, the call is aborted.
//   - The response always includes the self node.
//   - If the predecessor is not known yet, the field is nil.
//   - Successor and De Bruijn lists may contain fewer entries than
//     their configured maximum.
func (s *clientService) GetRoutingTable(ctx context.Context, _ *emptypb.Empty) (*clientv1.GetRoutingTableResponse, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	self := s.node.Self()
	pred := s.node.Predecessor()
	succList := s.node.SuccessorList()
	deBruijn := s.node.DeBruijnList()
	resp := &clientv1.GetRoutingTableResponse{
		Self:        self.ToProtoClient(),
		Predecessor: pred.ToProtoClient(),
	}
	for _, succ := range succList {
		resp.Successors = append(resp.Successors, succ.ToProtoClient())
	}
	for _, n := range deBruijn {
		resp.DeBruijnList = append(resp.DeBruijnList, n.ToProtoClient())
	}
	return resp, nil
}

// Lookup finds the node responsible for the given key.
//
// Errors:
//   - codes.InvalidArgument if the request is malformed or the ID is invalid
//   - codes.NotFound if no successor can be determined
//   - codes.Internal if the lookup fails due to internal errors
func (s *clientService) Lookup(
	ctx context.Context,
	req *clientv1.LookupRequest,
) (*clientv1.LookupResponse, error) {
	// Validate context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}

	// Validate request
	if req == nil || len(req.Id) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing ID")
	}
	id, err := s.node.Space().FromHexString(req.Id)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid ID")
	}

	// Enrich tracing span
	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(idAttributes("client.lookup.target", id)...)
	}

	// Perform lookup
	succ, err := s.node.LookUp(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "lookup failed: %v", err)
	}
	if succ == nil {
		return nil, status.Error(codes.NotFound, "no successor found")
	}

	// Convert to client-facing response
	return &clientv1.LookupResponse{
		Successor: succ.ToProtoClient(),
	}, nil
}
