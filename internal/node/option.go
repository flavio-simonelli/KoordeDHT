package node

import "KoordeDHT/internal/logger"

type Option func(*Node)

// WithLogger imposta il lgr usato dalla routing table.
func WithLogger(l logger.Logger) Option {
	return func(n *Node) {
		if l != nil {
			n.lgr = l
		}
	}
}
