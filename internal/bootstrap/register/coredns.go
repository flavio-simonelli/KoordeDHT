package register

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type CoreDNSRegistrar struct {
	client   *clientv3.Client
	basePath string
	domain   string
	ttl      int64
	leaseID  clientv3.LeaseID
}

// NewCoreDNSRegistrar creates a new Registrar backed by etcd/CoreDNS
func NewCoreDNSRegistrar(endpoints []string, basePath string, domain string, ttl int64) (*CoreDNSRegistrar, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &CoreDNSRegistrar{
		client:   cli,
		basePath: strings.TrimSuffix(basePath, "/"),
		domain:   strings.TrimSuffix(domain, "."),
		ttl:      ttl,
	}, nil
}

// Record stored in etcd
type record struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Priority int    `json:"priority"`
	Weight   int    `json:"weight"`
	TTL      int64  `json:"ttl,omitempty"`
}

func (r *CoreDNSRegistrar) makeSharedKey(nodeID string) string {
	return fmt.Sprintf("%s/dht/koorde/_tcp/_koorde/%s", r.basePath, nodeID)
}

func (r *CoreDNSRegistrar) RegisterNode(ctx context.Context, nodeID, targetHost string, port int) error {
	key := r.makeSharedKey(nodeID)
	rec := record{
		Host:     targetHost,
		Port:     port,
		Priority: 10,
		Weight:   100,
		TTL:      r.ttl,
	}
	val, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal record: %w", err)
	}

	lease, err := r.client.Grant(ctx, r.ttl)
	if err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}
	r.leaseID = lease.ID

	_, err = r.client.Put(ctx, key, string(val), clientv3.WithLease(lease.ID))
	if err != nil {
		return fmt.Errorf("put record: %w", err)
	}
	return nil
}

func (r *CoreDNSRegistrar) DeregisterNode(ctx context.Context, nodeID, targetHost string, port int) error {
	key := r.makeSharedKey(nodeID)
	_, err := r.client.Delete(ctx, key)
	return err
}

func (r *CoreDNSRegistrar) RenewNode(ctx context.Context, nodeID, targetHost string, port int) error {
	if r.leaseID == 0 {
		return fmt.Errorf("no active lease, call RegisterNode first")
	}
	_, err := r.client.KeepAliveOnce(ctx, r.leaseID)
	return err
}

func (r *CoreDNSRegistrar) Close() error {
	return r.client.Close()
}
