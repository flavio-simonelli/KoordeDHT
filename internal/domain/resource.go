package domain

import "errors"

var (
	ErrResourceNotFound = errors.New("resource not found")
)

type Resource struct {
	Key   ID
	Value string
}
