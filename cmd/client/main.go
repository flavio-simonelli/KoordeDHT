package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	clientv1 "KoordeDHT/internal/api/client/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func main() {
	// CLI flags
	addr := flag.String("addr", "127.0.0.1:5000", "Address of the Koorde node (entry point)")
	timeout := flag.Duration("timeout", 5*time.Second, "Request timeout")
	flag.Parse()

	// At least one command required
	if flag.NArg() < 1 {
		fmt.Println("Usage: koorde-client [--addr ip:port] put|get|delete <args>")
		os.Exit(1)
	}
	cmd := flag.Arg(0)

	// Setup gRPC connection
	conn, err := grpc.NewClient(
		*addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // plaintext, no TLS
	)
	if err != nil {
		log.Fatalf("Failed to connect to node at %s: %v", *addr, err)
	}
	defer func(conn *grpc.ClientConn) {
		_ = conn.Close()
	}(conn)

	client := clientv1.NewClientAPIClient(conn)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Dispatch commands
	switch cmd {
	case "put":
		if flag.NArg() < 3 {
			fmt.Println("Usage: koorde-client put <key> <value>")
			os.Exit(1)
		}
		key := flag.Arg(1)
		value := flag.Arg(2)
		req := &clientv1.PutRequest{Key: key, Value: value}
		_, err := client.Put(ctx, req)
		if err != nil {
			log.Fatalf("Put failed: %v", err)
		}
		fmt.Printf("Put succeeded (key=%s, value=%s)\n", key, value)

	case "get":
		if flag.NArg() < 2 {
			fmt.Println("Usage: koorde-client get <key>")
			os.Exit(1)
		}
		key := flag.Arg(1)
		req := &clientv1.GetRequest{Key: key}
		resp, err := client.Get(ctx, req)
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code().String() == "NotFound" {
				fmt.Printf("Key not found: %s\n", key)
				os.Exit(1)
			}
			log.Fatalf("Get failed: %v", err)
		}
		fmt.Printf("Get succeeded: key=%s, value=%s\n", key, resp.Value)

	case "delete":
		if flag.NArg() < 2 {
			fmt.Println("Usage: koorde-client delete <key>")
			os.Exit(1)
		}
		key := flag.Arg(1)
		req := &clientv1.DeleteRequest{Key: key}
		_, err := client.Delete(ctx, req)
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code().String() == "NotFound" {
				fmt.Printf("Key not found: %s\n", key)
				os.Exit(1)
			}
			log.Fatalf("Delete failed: %v", err)
		}
		fmt.Printf("Delete succeeded: key=%s\n", key)

	default:
		fmt.Println("Unknown command:", cmd)
		fmt.Println("Usage: koorde-client put|get|delete <args>")
		os.Exit(1)
	}
}
