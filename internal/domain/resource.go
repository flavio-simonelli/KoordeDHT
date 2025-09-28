package domain

import "errors"

var (
	ErrResourceNotFound = errors.New("resource not found")
	ErrNotResponsible   = errors.New("node not responsible for the given key")
)

type Resource struct {
	Key    ID
	RawKey string
	Value  string
}
