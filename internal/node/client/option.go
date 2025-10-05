package client

import (
	"KoordeDHT/internal/logger"
)

type Option func(pool *Pool)

// WithLogger sets a custom logger for the Pool.
func WithLogger(l logger.Logger) Option {
	return func(p *Pool) {
		p.lgr = l
	}
}
