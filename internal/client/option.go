package client

import (
	"KoordeDHT/internal/logger"
)

type Option func(pool *Pool)

// WithLogger imposta il logger usato dalla routing table.
func WithLogger(l logger.Logger) Option {
	return func(p *Pool) {
		p.lgr = l
	}
}
