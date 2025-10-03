package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"log"
	"math/big"
	"time"

	clientv1 "KoordeDHT/internal/api/client/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func randomHexBits(bits int) string {
	bytes := (bits + 7) / 8
	b := make([]byte, bytes)
	rand.Read(b)
	rem := bits % 8
	if rem != 0 {
		mask := byte((1<<rem - 1) << (8 - rem))
		b[0] &= mask
	}
	return hex.EncodeToString(b)
}

func pickRandom(nodes []string) string {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(nodes))))
	return nodes[n.Int64()]
}

func fetchRoutingTable(addr string, timeout time.Duration) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := clientv1.NewClientAPIClient(conn)
	rt, err := client.GetRoutingTable(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}

	var nodes []string
	if rt.Self != nil {
		nodes = append(nodes, rt.Self.Addr)
	}
	if rt.Predecessor != nil {
		nodes = append(nodes, rt.Predecessor.Addr)
	}
	for _, s := range rt.Successors {
		nodes = append(nodes, s.Addr)
	}
	for _, d := range rt.DeBruijnList {
		nodes = append(nodes, d.Addr)
	}
	return nodes, nil
}

func main() {
	bootstrap := flag.String("bootstrap", "127.0.0.1:5000", "bootstrap node address")
	bits := flag.Int("bits", 128, "ID length in bits")
	rate := flag.Float64("rate", 1.0, "lookup requests per second")
	timeout := flag.Duration("timeout", 2*time.Second, "per-request timeout")
	refresh := flag.Duration("refresh", 30*time.Second, "refresh routing table interval")
	flag.Parse()

	nodes, err := fetchRoutingTable(*bootstrap, *timeout)
	if err != nil || len(nodes) == 0 {
		log.Fatalf("Failed to fetch routing table from bootstrap %s: %v", *bootstrap, err)
	}
	log.Printf("Bootstrap succeeded, discovered %d nodes", len(nodes))

	interval := time.Duration(float64(time.Second) / *rate)
	ticker := time.NewTicker(*refresh)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// refresh with a random node
			n := pickRandom(nodes)
			newNodes, err := fetchRoutingTable(n, *timeout)
			if err == nil && len(newNodes) > 0 {
				nodes = newNodes
				log.Printf("Refreshed node list, now have %d nodes", len(nodes))
			}
		default:
			// perform one lookup
			id := randomHexBits(*bits)
			n := pickRandom(nodes)

			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			start := time.Now()
			conn, err := grpc.DialContext(ctx, n, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
			if err != nil {
				log.Printf("dial %s failed: %v", n, err)
				cancel()
				time.Sleep(interval)
				continue
			}
			client := clientv1.NewClientAPIClient(conn)
			_, err = client.Lookup(ctx, &clientv1.LookupRequest{Id: id})
			if err != nil {
				log.Printf("[lookup] id=%s via %s ERROR: %v latency=%s", id, n, err, time.Since(start))
			} else {
				log.Printf("[lookup] id=%s via %s OK latency=%s", id, n, time.Since(start))
			}
			conn.Close()
			cancel()

			time.Sleep(interval)
		}
	}
}
