package server

import "KoordeDHT/internal/logger"

// Option is a functional option for configuring the Server.
type Option func(*Server)

// WithLogger injects a custom logger into the Server.
func WithLogger(lgr logger.Logger) Option {
	return func(s *Server) {
		s.lgr = lgr
	}
}
