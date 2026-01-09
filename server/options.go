package server

import (
	"log/slog"

	"github.com/sig-0/fxrates/server/config"
)

type Option func(s *Server)

// WithLogger specifies the logger for the server
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) {
		s.logger = l
	}
}

// WithConfig specifies the config for the server
func WithConfig(c *config.Config) Option {
	return func(s *Server) {
		s.config = c
	}
}
