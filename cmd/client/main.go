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
	"google.golang.org/protobuf/types/known/emptypb"
)

func main() {
	// CLI flags
	addr := flag.String("addr", "127.0.0.1:5000", "Address of the Koorde node (entry point)")
	timeout := flag.Duration("timeout", 5*time.Second, "Request timeout")
	flag.Parse()

	// At least one command required
	if flag.NArg() < 1 {
		fmt.Println("Usage: koorde-client [--addr ip:port] put|get|delete|getstore|getrt|lookup <args>")
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
		req := &clientv1.PutRequest{
			Resource: &clientv1.Resource{Key: key, Value: value},
		}

		start := time.Now()
		_, err := client.Put(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			log.Fatalf("Put failed: %v", err)
		}
		fmt.Printf("Put succeeded (key=%s, value=%s) | latency=%s\n", key, value, elapsed)

	case "get":
		if flag.NArg() < 2 {
			fmt.Println("Usage: koorde-client get <key>")
			os.Exit(1)
		}
		key := flag.Arg(1)
		req := &clientv1.GetRequest{Key: key}

		start := time.Now()
		resp, err := client.Get(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code().String() == "NotFound" {
				fmt.Printf("Key not found: %s | latency=%s\n", key, elapsed)
				os.Exit(1)
			}
			log.Fatalf("Get failed: %v", err)
		}
		fmt.Printf("Get succeeded (key=%s, value=%s) | latency=%s\n", key, resp.Value, elapsed)

	case "delete":
		if flag.NArg() < 2 {
			fmt.Println("Usage: koorde-client delete <key>")
			os.Exit(1)
		}
		key := flag.Arg(1)
		req := &clientv1.DeleteRequest{Key: key}

		start := time.Now()
		_, err := client.Delete(ctx, req)
		elapsed := time.Since(start)

		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code().String() == "NotFound" {
				fmt.Printf("Key not found: %s | latency=%s\n", key, elapsed)
				os.Exit(1)
			}
			log.Fatalf("Delete failed: %v", err)
		}
		fmt.Printf("Delete succeeded (key=%s) | latency=%s\n", key, elapsed)

	case "getstore":
		// Streaming request
		stream, err := client.GetStore(ctx, &emptypb.Empty{})
		if err != nil {
			log.Fatalf("GetStore failed: %v", err)
		}

		fmt.Println("Stored resources on node:")
		for {
			resp, err := stream.Recv()
			if err != nil {
				break // EOF or error
			}
			if resp.GetItem() != nil {
				fmt.Printf("  - id=%s | key=%s | value=%s\n",
					resp.Id,
					resp.Item.Key,
					resp.Item.Value,
				)
			}
		}

	case "getrt":
		// Routing table request
		resp, err := client.GetRoutingTable(ctx, &emptypb.Empty{})
		if err != nil {
			log.Fatalf("GetRoutingTable failed: %v", err)
		}

		fmt.Println("Routing table:")

		if resp.Self != nil {
			fmt.Printf("  Self: %s (%s)\n", resp.Self.Id, resp.Self.Addr)
		} else {
			fmt.Println("  Self: nil")
		}

		if resp.Predecessor != nil {
			fmt.Printf("  Predecessor: %s (%s)\n", resp.Predecessor.Id, resp.Predecessor.Addr)
		} else {
			fmt.Println("  Predecessor: nil")
		}

		fmt.Println("  Successors:")
		for i, s := range resp.Successors {
			fmt.Printf("    [%d] %s (%s)\n", i, s.Id, s.Addr)
		}

		fmt.Println("  DeBruijn List:")
		for i, d := range resp.DeBruijnList {
			fmt.Printf("    [%d] %s (%s)\n", i, d.Id, d.Addr)
		}

	case "lookup":
		if flag.NArg() < 2 {
			fmt.Println("Usage: koorde-client lookup <id>")
			os.Exit(1)
		}
		id := flag.Arg(1)
		req := &clientv1.LookupRequest{Id: id}

		resp, err := client.Lookup(ctx, req)
		if err != nil {
			log.Fatalf("Lookup failed: %v", err)
		}

		fmt.Printf("Lookup result: %s (%s)\n", resp.Successor.Id, resp.Successor.Addr)
	}
}
