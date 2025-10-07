package main

import (
	"KoordeDHT/internal/client"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/peterh/liner"
)

func main() {
	// CLI flags
	addr := flag.String("addr", "bootstrap:4000", "Address of the Koorde node (entry point)")
	timeout := flag.Duration("timeout", 5*time.Second, "Request timeout (e.g., 5s)")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Connect to initial node
	api, conn, err := client.Connect(*addr)
	if err != nil {
		log.Fatalf("Failed to connect to node at %s: %v", *addr, err)
	}
	defer conn.Close()

	currentAddr := *addr
	fmt.Printf("Koorde interactive client. Connected to %s\n", currentAddr)
	fmt.Println("Available commands: put/get/delete/getstore/getrt/lookup/use/exit")

	// Setup liner shell
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)

	for {
		input, err := line.Prompt(fmt.Sprintf("koorde[%s]> ", currentAddr))
		if err != nil {
			if errors.Is(err, liner.ErrPromptAborted) {
				fmt.Println("Aborted")
				continue
			}
			break
		}
		line.AppendHistory(input)

		args := strings.Fields(strings.TrimSpace(input))
		if len(args) == 0 {
			continue
		}
		cmd := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), *timeout)

		switch cmd {

		case "put":
			if len(args) < 3 {
				fmt.Println("Usage: put <key> <value>")
				cancel()
				continue
			}
			key, value := args[1], args[2]
			delay, err := client.Put(ctx, api, key, value)
			if err != nil {
				fmt.Printf("Put failed (%v) | latency=%s\n", err, delay)
			} else {
				fmt.Printf("Put succeeded (key=%s, value=%s) | latency=%s\n", key, value, delay)
			}

		case "get":
			if len(args) < 2 {
				fmt.Println("Usage: get <key>")
				cancel()
				continue
			}
			key := args[1]
			val, delay, err := client.Get(ctx, api, key)
			switch err {
			case nil:
				fmt.Printf("Get succeeded (key=%s, value=%s) | latency=%s\n", key, val, delay)
			case client.ErrNotFound:
				fmt.Printf("Key not found: %s | latency=%s\n", key, delay)
			default:
				fmt.Printf("Get failed: %v | latency=%s\n", err, delay)
			}

		case "delete":
			if len(args) < 2 {
				fmt.Println("Usage: delete <key>")
				cancel()
				continue
			}
			key := args[1]
			delay, err := client.Delete(ctx, api, key)
			switch err {
			case nil:
				fmt.Printf("Delete succeeded (key=%s) | latency=%s\n", key, delay)
			case client.ErrNotFound:
				fmt.Printf("Key not found: %s | latency=%s\n", key, delay)
			default:
				fmt.Printf("Delete failed: %v | latency=%s\n", err, delay)
			}

		case "getstore":
			resources, delay, err := client.GetStore(ctx, api)
			if err != nil {
				fmt.Printf("GetStore failed: %v | latency=%s\n", err, delay)
				cancel()
				continue
			}
			fmt.Printf("Stored resources (count=%d) | latency=%s\n", len(resources), delay)
			for _, r := range resources {
				fmt.Printf("  - key=%s | value=%s\n", r.Key, r.Value)
			}

		case "getrt":
			rt, delay, err := client.GetRoutingTable(ctx, api)
			if err != nil {
				fmt.Printf("GetRoutingTable failed: %v | latency=%s\n", err, delay)
				cancel()
				continue
			}
			fmt.Println("Routing table:")
			if rt.Self != nil {
				fmt.Printf("  Self: %s (%s)\n", rt.Self.Id, rt.Self.Addr)
			}
			if rt.Predecessor != nil {
				fmt.Printf("  Predecessor: %s (%s)\n", rt.Predecessor.Id, rt.Predecessor.Addr)
			}
			fmt.Println("  Successors:")
			for i, s := range rt.Successors {
				fmt.Printf("    [%d] %s (%s)\n", i, s.Id, s.Addr)
			}
			fmt.Println("  DeBruijn List:")
			for i, d := range rt.DeBruijnList {
				fmt.Printf("    [%d] %s (%s)\n", i, d.Id, d.Addr)
			}
			fmt.Printf("Latency: %s\n", delay)

		case "lookup":
			if len(args) < 2 {
				fmt.Println("Usage: lookup <id>")
				cancel()
				continue
			}
			id := args[1]
			node, delay, err := client.Lookup(ctx, api, id)
			if err != nil {
				fmt.Printf("Lookup failed: %v | latency=%s\n", err, delay)
			} else {
				fmt.Printf("Lookup result: successor=%s (%s) | latency=%s\n",
					node.Id, node.Addr, delay)
			}

		case "use":
			if len(args) < 2 {
				fmt.Println("Usage: use <addr>")
				cancel()
				continue
			}
			newAddr := args[1]
			newClient, newConn, err := client.Connect(newAddr)
			if err != nil {
				fmt.Printf("Failed to connect to %s: %v\n", newAddr, err)
				cancel()
				continue
			}
			conn.Close()
			api = newClient
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
