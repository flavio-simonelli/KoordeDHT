package domain

import (
	clientv1 "KoordeDHT/internal/api/client/v1"
	dhtv1 "KoordeDHT/internal/api/dht/v1"
	"errors"
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
// protobuf representation (dht.v1.Resource).
func (r *Resource) ToProtoDHT() *dhtv1.Resource {
	if r == nil {
		return nil
	}
	return &dhtv1.Resource{
		Key:    r.Key,    // already []byte
		RawKey: r.RawKey, // debug only
		Value:  r.Value,
	}
}

// ResourceFromProtoDHT converts a DHT-facing resource into
// a domain.Resource.
func ResourceFromProtoDHT(sp *Space, p *dhtv1.Resource) (*Resource, error) {
	if p == nil {
		return nil, nil
	}
	if err := sp.IsValidID(p.Key); err != nil {
		return nil, errors.New("invalid resource key ID")
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
func ResourceFromProtoClient(sp *Space, p *clientv1.Resource) *Resource {
	if p == nil {
		return nil
	}
	key := sp.NewIdFromString(p.Key)
	return &Resource{
		RawKey: p.Key,
		Key:    key,
		Value:  p.Value,
	}
}
