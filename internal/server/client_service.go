package server

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
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
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	if len(req.Value) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing value")
	}
	err := s.node.Put(ctx, req.Key, req.Value)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// Get retrieves a resource by key. Returns NotFound if the key does not exist.
func (s *clientService) Get(ctx context.Context, req *clientv1.GetRequest) (*clientv1.GetResponse, error) {
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if req == nil || len(req.Key) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing key")
	}
	res, err := s.node.Get(ctx, req.Key)
	if err != nil {
		if errors.Is(err, domain.ErrResourceNotFound) {
			return nil, status.Error(codes.NotFound, "key not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &clientv1.GetResponse{Value: res.Value}, nil
}

// Delete removes a resource by key. Returns NotFound if the key does not exist.
func (s *clientService) Delete(ctx context.Context, req *clientv1.DeleteRequest) (*emptypb.Empty, error) {
	// Check for canceled/expired context
	if err := checkContext(ctx); err != nil {
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
