package client

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Connect(addr string) (clientv1.ClientAPIClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	return clientv1.NewClientAPIClient(conn), conn, nil
}
