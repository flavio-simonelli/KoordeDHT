package storage

import (
	"sync"

	"KoordeDHT/internal/domain"
)

type MemoryStorage struct {
	mu   sync.RWMutex
	data map[string]string // chiavi salvate come stringhe hex dell’ID
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[string]string),
	}
}

func (m *MemoryStorage) Put(id domain.ID, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[id.ToHexString()] = value
	return nil
}

func (m *MemoryStorage) Get(id domain.ID) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[id.ToHexString()]
	if !ok {
		return "", ErrNotFound
	}
	return val, nil
}

func (m *MemoryStorage) Delete(id domain.ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, id.ToHexString())
	return nil
}

func (m *MemoryStorage) Between(from, to domain.ID) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range m.data {
		id, err := domain.FromHexString(k, len(from)*8)
		if err != nil {
			continue
		}
		if id.InOC(from, to) {
			result[id.ToHexString()] = v
		}
	}
	return result, nil
}
