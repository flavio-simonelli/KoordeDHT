package node

import "KoordeDHT/internal/logger"

type Option func(*Node)

// WithLogger imposta il logger usato dalla routing table.
func WithLogger(l logger.Logger) Option {
	return func(n *Node) {
		if l != nil {
			n.logger = l
		}
	}
}
