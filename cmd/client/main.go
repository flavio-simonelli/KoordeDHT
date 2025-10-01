package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	clientv1 "KoordeDHT/internal/api/client/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func connect(addr string) (clientv1.ClientAPIClient, *grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, err
	}
	return clientv1.NewClientAPIClient(conn), conn, nil
}

func main() {
	// Flags iniziali
	addr := flag.String("addr", "127.0.0.1:5000", "Address of the Koorde node (entry point)")
	timeout := flag.Duration("timeout", 5*time.Second, "Request timeout")
	flag.Parse()

	// Connessione iniziale
	client, conn, err := connect(*addr)
	if err != nil {
		log.Fatalf("Failed to connect to node at %s: %v", *addr, err)
	}
	defer conn.Close()

	currentAddr := *addr
	reader := bufio.NewScanner(os.Stdin)

	fmt.Printf("Koorde interactive client. Connected to %s\n", currentAddr)
	fmt.Println("Available commands: put/get/delete/getstore/getrt/lookup/use/exit")

	for {
		fmt.Printf("koorde[%s]> ", currentAddr)
		if !reader.Scan() {
			break
		}
		line := strings.TrimSpace(reader.Text())
		if line == "" {
			continue
		}

		args := strings.Fields(line)
		cmd := args[0]

		// Context per ogni comando
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		start := time.Now()

		switch cmd {

		case "put":
			if len(args) < 3 {
				fmt.Println("Usage: put <key> <value>")
				cancel()
				continue
			}
			key, value := args[1], args[2]
			req := &clientv1.PutRequest{
				Resource: &clientv1.Resource{Key: key, Value: value},
			}
			_, err := client.Put(ctx, req)
			if err != nil {
				log.Printf("Put failed: %v\n", err)
			} else {
				fmt.Printf("Put succeeded (key=%s, value=%s) | latency=%s\n", key, value, time.Since(start))
			}

		case "get":
			if len(args) < 2 {
				fmt.Println("Usage: get <key>")
				cancel()
				continue
			}
			key := args[1]
			req := &clientv1.GetRequest{Key: key}
			resp, err := client.Get(ctx, req)
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code().String() == "NotFound" {
					fmt.Printf("Key not found: %s | latency=%s\n", key, time.Since(start))
				} else {
					log.Printf("Get failed: %v\n", err)
				}
			} else {
				fmt.Printf("Get succeeded (key=%s, value=%s) | latency=%s\n", key, resp.Value, time.Since(start))
			}

		case "delete":
			if len(args) < 2 {
				fmt.Println("Usage: delete <key>")
				cancel()
				continue
			}
			key := args[1]
			req := &clientv1.DeleteRequest{Key: key}
			_, err := client.Delete(ctx, req)
			if err != nil {
				if s, ok := status.FromError(err); ok && s.Code().String() == "NotFound" {
					fmt.Printf("Key not found: %s | latency=%s\n", key, time.Since(start))
				} else {
					log.Printf("Delete failed: %v\n", err)
				}
			} else {
				fmt.Printf("Delete succeeded (key=%s) | latency=%s\n", key, time.Since(start))
			}

		case "getstore":
			stream, err := client.GetStore(ctx, &emptypb.Empty{})
			if err != nil {
				log.Printf("GetStore failed: %v\n", err)
				cancel()
				continue
			}
			fmt.Println("Stored resources on node:")
			for {
				resp, err := stream.Recv()
				if err != nil {
					break
				}
				if resp.GetItem() != nil {
					fmt.Printf("  - id=%s | key=%s | value=%s\n",
						resp.Id, resp.Item.Key, resp.Item.Value)
				}
			}

		case "getrt":
			resp, err := client.GetRoutingTable(ctx, &emptypb.Empty{})
			if err != nil {
				log.Printf("GetRoutingTable failed: %v\n", err)
				cancel()
				continue
			}
			fmt.Println("Routing table:")
			if resp.Self != nil {
				fmt.Printf("  Self: %s (%s)\n", resp.Self.Id, resp.Self.Addr)
			}
			if resp.Predecessor != nil {
				fmt.Printf("  Predecessor: %s (%s)\n", resp.Predecessor.Id, resp.Predecessor.Addr)
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
			if len(args) < 2 {
				fmt.Println("Usage: lookup <id>")
				cancel()
				continue
			}
			id := args[1]
			req := &clientv1.LookupRequest{Id: id}
			resp, err := client.Lookup(ctx, req)
			if err != nil {
				log.Printf("Lookup failed: %v | latency=%s\n", err, time.Since(start))
			} else {
				fmt.Printf("Lookup result: %s (%s) | latency=%s\n",
					resp.Successor.Id, resp.Successor.Addr, time.Since(start))
			}

		case "use":
			if len(args) < 2 {
				fmt.Println("Usage: use <addr>")
				cancel()
				continue
			}
			newAddr := args[1]
			newClient, newConn, err := connect(newAddr)
			if err != nil {
				log.Printf("Failed to connect to %s: %v\n", newAddr, err)
				cancel()
				continue
			}
			conn.Close()
			client = newClient
			conn = newConn
			currentAddr = newAddr
			fmt.Printf("Switched connection to %s\n", currentAddr)

		case "exit", "quit":
			fmt.Println("Bye!")
			cancel()
			return

		default:
			fmt.Printf("Unknown command: %s\n", cmd)
			fmt.Println("Available: put/get/delete/getstore/getrt/lookup/use/exit")
		}

		cancel()
	}
}
