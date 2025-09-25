package storage

import (
	"KoordeDHT/internal/domain"
	"sync"
)

// memoryStorage is an in-memory key-value store that implements the Storage
// interface. It is concurrency-safe and intended for local node storage.
type memoryStorage struct {
	mu   sync.RWMutex
	data map[string]domain.Resource // chiave = ID esadecimale
}

// NewMemoryStorage creates and returns a new, empty in-memory storage.
// This implementation is suitable for unit tests and for nodes that do not
// require persistence.
func NewMemoryStorage() Storage {
	return &memoryStorage{
		data: make(map[string]domain.Resource),
	}
}

// Put inserts or updates the given resource in the store.
// The resource is indexed by its ID, serialized as a hexadecimal string.
// Returns nil in all cases (reserved for future implementations).
func (s *memoryStorage) Put(resource domain.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[resource.Key.String()] = resource
	return nil
}

// Get retrieves the resource with the given ID.
// If the key is not present, it returns ErrNotFound.
func (s *memoryStorage) Get(id domain.ID) (domain.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.data[id.String()]
	if !ok {
		return domain.Resource{}, ErrNotFound
	}
	return res, nil
}

// Delete removes the resource with the given ID from the store.
// If the key is not present, it returns ErrNotFound.
func (s *memoryStorage) Delete(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id.String()]; !ok {
		return ErrNotFound
	}
	delete(s.data, id.String())
	return nil
}

// Between returns all resources with IDs k such that k ∈ (from, to] on the ring.
// The wrap-around case (from > to) is correctly handled by domain.ID.Between.
func (s *memoryStorage) Between(from, to domain.ID) ([]domain.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []domain.Resource
	for _, res := range s.data {
		if res.Key.Between(from, to) {
			result = append(result, res)
		}
	}
	return result, nil
}

// All returns a snapshot of all resources currently stored.
// The slice is a copy and modifications to it do not affect the storage.
func (s *memoryStorage) All() ([]domain.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.Resource, 0, len(s.data))
	for _, res := range s.data {
		result = append(result, res)
	}
	return result, nil
}
