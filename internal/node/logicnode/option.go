package logicnode

import "KoordeDHT/internal/logger"

type Option func(*Node)

// WithLogger sets a custom logger for the Node.
func WithLogger(l logger.Logger) Option {
	return func(n *Node) {
		if l != nil {
			n.lgr = l
		}
	}
}
