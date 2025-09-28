package storage

import (
	"KoordeDHT/internal/domain"
	"KoordeDHT/internal/logger"
	"sort"
	"sync"
)

// Storage is an in-memory key-value store that implements the Storage
// interface. It is concurrency-safe and intended for local node storage.
type Storage struct {
	lgr  logger.Logger
	mu   sync.RWMutex
	data map[string]domain.Resource // chiave = ID esadecimale
}

// NewMemoryStorage creates and returns a new, empty in-memory storage.
// This implementation is suitable for unit tests and for nodes that do not
// require persistence.
func NewMemoryStorage(lgr logger.Logger) *Storage {
	s := &Storage{
		lgr:  lgr,
		data: make(map[string]domain.Resource),
	}
	s.lgr.Debug("initialized routing table")
	return s
}

// Put inserts or updates the given resource in the store.
// The resource is indexed by its ID, serialized as a hexadecimal string.
// Returns nil in all cases (reserved for future implementations).
func (s *Storage) Put(resource domain.Resource) {
	key := resource.Key.String()
	s.mu.Lock()
	_, existed := s.data[key]
	s.data[key] = resource
	s.mu.Unlock()
	if existed {
		s.lgr.Debug("Put: resource updated", logger.FResource("resource", resource))
	} else {
		s.lgr.Debug("Put: resource inserted", logger.FResource("resource", resource))
	}
}

// Get retrieves the resource with the given ID.
// If the key is not present, it returns ErrResourceNotFound.
func (s *Storage) Get(id domain.ID) (domain.Resource, error) {
	key := id.String()

	s.mu.RLock()
	res, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		s.lgr.Debug("Get: resource not found", logger.F("key", key))
		return domain.Resource{}, domain.ErrResourceNotFound
	}
	s.lgr.Debug("Get: resource retrieved", logger.FResource("resource", res))
	return res, nil
}

// Delete removes the resource with the given ID from the store.
// If the key is not present, it returns ErrResourceNotFound.
func (s *Storage) Delete(id domain.ID) error {
	key := id.String()
	s.mu.Lock()
	_, ok := s.data[key]
	if ok {
		delete(s.data, key)
	}
	s.mu.Unlock()
	if !ok {
		s.lgr.Debug("Storage: delete failed, resource not found", logger.F("key", key))
		return domain.ErrResourceNotFound
	}
	s.lgr.Debug("Storage: resource deleted", logger.F("key", key))
	return nil
}

// Between returns all resources with IDs k such that k ∈ (from, to] on the ring.
// The wrap-around case (from > to) is correctly handled by domain.ID.Between.
func (s *Storage) Between(from, to domain.ID) ([]domain.Resource, error) {
	s.mu.RLock()
	var result []domain.Resource
	for _, res := range s.data {
		if res.Key.Between(from, to) {
			result = append(result, res)
		}
	}
	s.mu.RUnlock()
	// prepare compact list of keys for logging
	keys := make([]string, 0, len(result))
	for _, r := range result {
		keys = append(keys, r.Key.String())
	}
	s.lgr.Debug("Storage: range query completed",
		logger.F("from", from.String()),
		logger.F("to", to.String()),
		logger.F("count", len(result)),
		logger.F("keys", keys),
	)
	return result, nil
}

// All returns a snapshot of all resources currently stored.
// The slice is a copy and modifications to it do not affect the storage.
func (s *Storage) All() []domain.Resource {
	s.mu.RLock()
	result := make([]domain.Resource, 0, len(s.data))
	for _, res := range s.data {
		result = append(result, res)
	}
	s.mu.RUnlock()
	// prepare compact list of keys for logging
	keys := make([]string, 0, len(result))
	for _, r := range result {
		keys = append(keys, r.Key.String())
	}
	s.lgr.Debug("Storage: snapshot retrieved",
		logger.F("count", len(result)),
		logger.F("keys", keys),
	)
	return result
}

// DebugLog emits a structured DEBUG-level log with the contents of the storage.
//
// The log entry includes:
//   - A count of stored resources
//   - An ordered list of resources (key + value)
//
// This method does not print to stdout. It is intended for debugging and
// monitoring; the storage contents are read under a read lock and logged
// as a snapshot without modifying the data.
func (s *Storage) DebugLog() {
	s.mu.RLock()
	snapshot := make([]domain.Resource, 0, len(s.data))
	for _, res := range s.data {
		snapshot = append(snapshot, res)
	}
	s.mu.RUnlock()
	// Sort by key for deterministic order
	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].Key.String() < snapshot[j].Key.String()
	})
	entries := make([]map[string]any, 0, len(snapshot))
	for _, res := range snapshot {
		entries = append(entries, map[string]any{
			"key":   res.Key.String(),
			"value": res.Value,
		})
	}
	s.lgr.Debug("Storage snapshot",
		logger.F("count", len(snapshot)),
		logger.F("resources", entries),
	)
}
