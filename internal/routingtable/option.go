package routingtable

import "KoordeDHT/internal/logger"

type Option func(*RoutingTable)

// WithLogger imposta il logger usato dalla routing table.
func WithLogger(l logger.Logger) Option {
	return func(rt *RoutingTable) {
		rt.logger = l
	}
}
