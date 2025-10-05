package client

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	"context"
	"errors"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	ErrNotFound         = errors.New("resource not found")
	ErrUnavailable      = errors.New("node unavailable")
	ErrDeadlineExceeded = errors.New("request timeout exceeded")
	ErrInternal         = errors.New("internal gRPC error")
)

// normalizeError converts a gRPC status error into a common internal error.
func normalizeError(err error) error {
	if err == nil {
		return nil
	}

	s, ok := status.FromError(err)
	if !ok {
		return ErrInternal
	}

	switch s.Code() {
	case codes.NotFound:
		return ErrNotFound
	case codes.Unavailable:
		return ErrUnavailable
	case codes.DeadlineExceeded:
		return ErrDeadlineExceeded
	default:
		return ErrInternal
	}
}

// Put inserts or updates a key-value pair on the node.
func Put(ctx context.Context, client clientv1.ClientAPIClient, key, value string) (time.Duration, error) {
	start := time.Now()
	_, err := client.Put(ctx, &clientv1.PutRequest{
		Resource: &clientv1.Resource{Key: key, Value: value},
	})
	return time.Since(start), normalizeError(err)
}

// Get retrieves the value for a given key.
func Get(ctx context.Context, client clientv1.ClientAPIClient, key string) (string, time.Duration, error) {
	start := time.Now()
	resp, err := client.Get(ctx, &clientv1.GetRequest{Key: key})
	if err != nil {
		return "", time.Since(start), normalizeError(err)
	}
	return resp.Value, time.Since(start), nil
}

// Delete removes a key from the node.
func Delete(ctx context.Context, client clientv1.ClientAPIClient, key string) (time.Duration, error) {
	start := time.Now()
	_, err := client.Delete(ctx, &clientv1.DeleteRequest{Key: key})
	return time.Since(start), normalizeError(err)
}

// Lookup performs a DHT lookup by ID and returns the successor node.
func Lookup(ctx context.Context, client clientv1.ClientAPIClient, id string) (*clientv1.NodeInfo, time.Duration, error) {
	start := time.Now()
	resp, err := client.Lookup(ctx, &clientv1.LookupRequest{Id: id})
	if err != nil {
		return nil, time.Since(start), normalizeError(err)
	}
	return resp.Successor, time.Since(start), nil
}

// GetRoutingTable retrieves the node’s routing table.
func GetRoutingTable(ctx context.Context, client clientv1.ClientAPIClient) (*clientv1.GetRoutingTableResponse, time.Duration, error) {
	start := time.Now()
	resp, err := client.GetRoutingTable(ctx, &emptypb.Empty{})
	return resp, time.Since(start), normalizeError(err)
}

// GetStore streams all key-value pairs stored in the node.
func GetStore(ctx context.Context, client clientv1.ClientAPIClient) ([]*clientv1.Resource, time.Duration, error) {
	start := time.Now()
	stream, err := client.GetStore(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, 0, normalizeError(err)
	}

	var resources []*clientv1.Resource
	for {
		resp, recvErr := stream.Recv()
		if recvErr != nil {
			break
		}
		if resp.GetItem() != nil {
			resources = append(resources, resp.Item)
		}
	}
	return resources, time.Since(start), nil
}
