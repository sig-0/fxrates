package ingest

import (
	"context"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

// Provider is a single custom exchange rate provider
type Provider interface {
	// Name returns the human-readable name of the provider
	Name() string

	// Interval returns the interval at which the provider should be called
	Interval() time.Duration

	// Fetch is the provider's main fetch job, yielding exchange rate data points
	Fetch(context.Context) ([]*types.ExchangeRate, error)
}
