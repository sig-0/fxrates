package storage

import (
	"context"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

// Storage is an abstraction over exchange rate data
type Storage interface {
	// SaveExchangeRate saves the given exchange rate data point
	SaveExchangeRate(context.Context, *types.ExchangeRate) error

	// RateAsOf fetches the rate as of the given time
	RateAsOf(context.Context, *types.RateQuery, time.Time) (*types.ExchangeRate, error)

	// RatesInRange fetches the rates as a paginated series in the given timeframe
	RatesInRange(
		context.Context,
		*types.RateQuery,
		time.Time, // from
		time.Time, // to
		int32, // limit
		int64, // offset
	) (*types.Page[*types.ExchangeRate], error)

	// ListSources lists all present sources for fx rates
	ListSources(context.Context) ([]types.Source, error)

	// ListCurrencies lists all currencies present
	ListCurrencies(context.Context) ([]types.Currency, error)
}
