package routingtable

import "KoordeDHT/internal/logger"

type Option func(*RoutingTable)

// WithLogger sets a custom logger for the RoutingTable.
func WithLogger(l logger.Logger) Option {
	return func(rt *RoutingTable) {
		rt.logger = l
	}
}
