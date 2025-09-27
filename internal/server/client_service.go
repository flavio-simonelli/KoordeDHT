package server

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	"KoordeDHT/internal/ctxutil"
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/node"
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// clientService implements the ClientServiceServer interface.
type clientService struct {
	clientv1.UnimplementedClientAPIServer
	node *node.Node
}

func NewClientService(n *node.Node) clientv1.ClientAPIServer {
	return &clientService{node: n}
}

// Put handles the Put RPC call.
func (s *clientService) Put(ctx context.Context, req *clientv1.PutRequest) (*emptypb.Empty, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || len(req.Resource.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	if len(req.Resource.Value) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing value")
	}
	err := s.node.Put(ctx, req.Resource.Key, req.Resource.Value)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Get retrieves a resource by key. Returns NotFound if the key does not exist.
func (s *clientService) Get(ctx context.Context, req *clientv1.GetRequest) (*clientv1.GetResponse, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	res, err := s.node.Get(ctx, req.Key)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "resource not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if res == nil {
		return nil, status.Error(codes.NotFound, "resource not found")
	}
	return &clientv1.GetResponse{Value: res.Value}, nil
}

// Delete removes a resource by key. Returns NotFound if the key does not exist.
func (s *clientService) Delete(ctx context.Context, req *clientv1.DeleteRequest) (*emptypb.Empty, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	err := s.node.Delete(ctx, req.Key)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "key not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &emptypb.Empty{}, nil
}

// GetStore returns all key-value pairs stored on this node.
func (s *clientService) GetStore(_ *emptypb.Empty, stream clientv1.ClientAPI_GetStoreServer) error {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(stream.Context()); err != nil {
		return err
	}
	resources := s.node.GetAllResourceStored()
	for _, r := range resources {
		if err := ctxutil.CheckContext(stream.Context()); err != nil {
			return err
		}
		res := &clientv1.GetStoreResponse{
			Item: &clientv1.Resource{
				Key:   r.Key.String(),
				Value: r.Value,
			},
		}
		if err := stream.Send(res); err != nil {
			return status.Error(codes.Internal, "failed to send resource: "+err.Error())
		}
	}
	return nil
}

// GetRoutingTable returns the current routing table of the node.
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
func (s *clientService) Lookup(ctx context.Context, req *clientv1.LookupRequest) (*clientv1.LookupResponse, error) {
	// Check for canceled/expired context
	if err := ctxutil.CheckContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || len(req.Id) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing ID")
	}
	succ, err := s.node.LookUp(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if succ == nil {
		return nil, status.Error(codes.NotFound, "no successor found")
	}
	return &clientv1.LookupResponse{Successor: succ.ToProtoClient()}, nil
}
