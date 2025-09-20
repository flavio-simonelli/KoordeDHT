package storage

import (
	"errors"

	"KoordeDHT/internal/domain"
)

var (
	ErrNotFound = errors.New("key not found")
)

// Storage definisce le operazioni minime per la DHT
type Storage interface {
	// Put inserisce o aggiorna una chiave
	Put(id domain.ID, value string) error

	// Get restituisce il valore associato a una chiave
	Get(id domain.ID) (string, error)

	// Delete rimuove una chiave
	Delete(id domain.ID) error

	// Between restituisce tutte le coppie (k,v) con k ∈ (from, to]
	Between(from, to domain.ID) (map[string]string, error)
}
