package ingest

import (
	"log/slog"
	"time"
)

type Option func(o *Orchestrator)

// WithLogger specifies the logger for the orchestrator
func WithLogger(l *slog.Logger) Option {
	return func(o *Orchestrator) {
		o.logger = l
	}
}

// WithQueryInterval specifies query interval for the orchestrator's jobs.
// Defaults to 1s.
// This should only be modified if the registered providers with the orchestrator
// have sparse runs (once every hour / 24hrs)
func WithQueryInterval(q time.Duration) Option {
	return func(o *Orchestrator) {
		o.queryInterval = q
	}
}
