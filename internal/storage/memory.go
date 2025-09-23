package storage

import (
	"KoordeDHT/internal/logger"
	"sync"

	"KoordeDHT/internal/domain"
)

type MemoryStorage struct {
	lgr  logger.Logger
	mu   sync.RWMutex
	data map[string]string // chiavi salvate come stringhe hex dell’ID (perchè non si può confrontare fra slice, quindi dovrei fissare la dimensione degli id)
}

func NewMemoryStorage(log logger.Logger) *MemoryStorage {
	return &MemoryStorage{
		lgr:  log,
		data: make(map[string]string),
	}
}

func (m *MemoryStorage) Put(resource domain.Resource) error {
	m.mu.Lock()
	m.data[resource.Key.Hex()] = resource.Value
	m.mu.Unlock()
	m.lgr.Info("Add new Resource", logger.F("key", resource.Key.Hex()), logger.F("value", resource.Value))
	return nil
}

func (m *MemoryStorage) Get(id domain.ID) (domain.Resource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[id.Hex()]
	if !ok {
		return domain.Resource{}, ErrNotFound
	}
	res := domain.Resource{
		Key:   id,
		Value: val,
	}
	return res, nil
}

func (m *MemoryStorage) Delete(id domain.ID) error {
	m.mu.Lock()
	delete(m.data, id.Hex())
	m.mu.Unlock()
	m.lgr.Info("Delete Resource", logger.F("key", id.Hex()))
	return nil
}

/*

func (m *MemoryStorage) Between(from, to domain.ID) ([]domain.Resource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Resource, 0)
	for k, v := range m.data {
		id, err := domain.(k, len(from)*8)
		if err != nil {
			continue
		}
		if id.InOC(from, to) {
			result = append(result, domain.Resource{
				Key:   id,
				Value: v,
			})
		}
	}
	return result, nil
}

*/
