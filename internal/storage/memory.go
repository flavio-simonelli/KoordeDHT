package storage

import (
	"KoordeDHT/internal/domain"
	"fmt"
	"os"
	"sort"
	"sync"
	"text/tabwriter"
)

// Storage is an in-memory key-value store that implements the Storage
// interface. It is concurrency-safe and intended for local node storage.
type Storage struct {
	mu   sync.RWMutex
	data map[string]domain.Resource // chiave = ID esadecimale
}

// NewMemoryStorage creates and returns a new, empty in-memory storage.
// This implementation is suitable for unit tests and for nodes that do not
// require persistence.
func NewMemoryStorage() *Storage {
	return &Storage{
		data: make(map[string]domain.Resource),
	}
}

// Put inserts or updates the given resource in the store.
// The resource is indexed by its ID, serialized as a hexadecimal string.
// Returns nil in all cases (reserved for future implementations).
func (s *Storage) Put(resource domain.Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[resource.Key.String()] = resource
	return nil
}

// Get retrieves the resource with the given ID.
// If the key is not present, it returns ErrNotFound.
func (s *Storage) Get(id domain.ID) (domain.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res, ok := s.data[id.String()]
	if !ok {
		return domain.Resource{}, domain.ErrResourceNotFound
	}
	return res, nil
}

// Delete removes the resource with the given ID from the store.
// If the key is not present, it returns ErrNotFound.
func (s *Storage) Delete(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id.String()]; !ok {
		return domain.ErrResourceNotFound
	}
	delete(s.data, id.String())
	return nil
}

// Between returns all resources with IDs k such that k ∈ (from, to] on the ring.
// The wrap-around case (from > to) is correctly handled by domain.ID.Between.
func (s *Storage) Between(from, to domain.ID) ([]domain.Resource, error) {
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
func (s *Storage) All() ([]domain.Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]domain.Resource, 0, len(s.data))
	for _, res := range s.data {
		result = append(result, res)
	}
	return result, nil
}

// DebugPrint prints the contents of the storage as a formatted table
// to stdout, with a clear header and separators to distinguish it from
// other debug dumps.
func (s *Storage) DebugPrint() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Print a distinct header
	fmt.Println("=== Storage Debug Dump ===")
	if len(s.data) == 0 {
		fmt.Println("Storage is empty.")
		return
	}

	// Setup tabwriter for aligned columns
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Table header
	fmt.Fprintln(w, "ID (hex)\tValue")

	// Collect and sort keys for deterministic order
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Print each resource
	for _, k := range keys {
		res := s.data[k]
		fmt.Fprintf(w, "%s\t%s\n", res.Key.String(), res.Value)
	}

	// Flush writer
	_ = w.Flush()
	fmt.Println("==========================")
}
