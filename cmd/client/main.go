package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	clientv1 "KoordeDHT/internal/api/client/v1"

	"github.com/peterh/liner"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func connect(addr string) (clientv1.ClientAPIClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}
	return clientv1.NewClientAPIClient(conn), conn, nil
}

func main() {
	addr := flag.String("addr", "bootstrap:4000", "Address of the Koorde node (entry point)")
	timeout := flag.Duration("timeout", 5*time.Second, "Request timeout")
	flag.Parse()

	client, conn, err := connect(*addr)
	if err != nil {
		log.Fatalf("Failed to connect to node at %s: %v", *addr, err)
	}
	defer conn.Close()

	currentAddr := *addr
	fmt.Printf("Koorde interactive client. Connected to %s\n", currentAddr)

	// setup liner for readline support
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)

	// prompt help
	fmt.Println("Available commands: put/get/delete/getstore/getrt/lookup/use/exit")

	for {
		input, err := line.Prompt(fmt.Sprintf("koorde[%s]> ", currentAddr))
		if err != nil {
			if err == liner.ErrPromptAborted {
				fmt.Println("Aborted")
				continue
			}
			break
		}

		line.AppendHistory(input) // salva in cronologia

		args := strings.Fields(strings.TrimSpace(input))
		if len(args) == 0 {
			continue
		}
		cmd := args[0]

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
			_, err := client.Put(ctx, &clientv1.PutRequest{
				Resource: &clientv1.Resource{Key: key, Value: value},
			})
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
			resp, err := client.Get(ctx, &clientv1.GetRequest{Key: key})
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
			_, err := client.Delete(ctx, &clientv1.DeleteRequest{Key: key})
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
			resp, err := client.Lookup(ctx, &clientv1.LookupRequest{Id: id})
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
		}

		cancel()
	}
}
