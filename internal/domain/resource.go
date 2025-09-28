package domain

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"errors"
	"fmt"
)

var (
	ErrResourceNotFound = errors.New("resource not found")
	ErrNotResponsible   = errors.New("node not responsible for the given key")
)

type Resource struct {
	Key    ID
	RawKey string
	Value  string
}

// ToProtoDHT converts a domain.Resource into its DHT-facing
// protobuf representation (dht.v1.StoreRequest).
func (r *Resource) ToProtoDHT() *dhtv1.StoreRequest {
	if r == nil {
		return nil
	}
	return &dhtv1.StoreRequest{
		Key:    r.Key,    // already []byte
		RawKey: r.RawKey, // debug only
		Value:  r.Value,
	}
}

// ResourceFromProtoDHT converts a DHT-facing resource into
// a domain.Resource, validating the key as a proper ID.
func ResourceFromProtoDHT(sp Space, p *dhtv1.StoreRequest) (*Resource, error) {
	if p == nil {
		return nil, nil
	}
	if err := sp.IsValidID(p.Key); err != nil {
		return nil, fmt.Errorf("invalid resource key: %w", err)
	}
	return &Resource{
		Key:    p.Key,
		RawKey: p.RawKey,
		Value:  p.Value,
	}, nil
}

// ToProtoClient converts a domain.Resource into its client-facing
// protobuf representation (client.v1.Resource).
func (r *Resource) ToProtoClient() *clientv1.Resource {
	if r == nil {
		return nil
	}
	return &clientv1.Resource{
		Key:   r.RawKey,
		Value: r.Value,
	}
}

// ResourceFromProtoClient converts a client-facing resource
// into a domain.Resource. The ID must be computed later
// by hashing the RawKey into the DHT space.
func ResourceFromProtoClient(p *clientv1.Resource) *Resource {
	if p == nil {
		return nil
	}
	return &Resource{
		RawKey: p.Key,
		Value:  p.Value,
	}
}
