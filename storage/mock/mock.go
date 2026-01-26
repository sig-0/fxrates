package mock

import (
	"context"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

type (
	SaveExchangeRateDelegate func(context.Context, *types.ExchangeRate) error
	RateAsOfDelegate         func(context.Context, *types.RateQuery, time.Time) (*types.Page[*types.ExchangeRate], error)
	ListSourcesDelegate      func(context.Context) ([]types.Source, error)
	ListCurrenciesDelegate   func(context.Context) ([]types.Currency, error)
)

type Storage struct {
	SaveExchangeRateFn SaveExchangeRateDelegate
	RateAsOfFn         RateAsOfDelegate
	ListSourcesFn      ListSourcesDelegate
	ListCurrenciesFn   ListCurrenciesDelegate
}

func (m *Storage) SaveExchangeRate(ctx context.Context, rate *types.ExchangeRate) error {
	if m.SaveExchangeRateFn != nil {
		return m.SaveExchangeRateFn(ctx, rate)
	}

	return nil
}

func (m *Storage) RateAsOf(
	ctx context.Context,
	query *types.RateQuery,
	at time.Time,
) (*types.Page[*types.ExchangeRate], error) {
	if m.RateAsOfFn != nil {
		return m.RateAsOfFn(ctx, query, at)
	}

	return nil, nil
}

func (m *Storage) ListSources(ctx context.Context) ([]types.Source, error) {
	if m.ListSourcesFn != nil {
		return m.ListSourcesFn(ctx)
	}

	return nil, nil
}

func (m *Storage) ListCurrencies(ctx context.Context) ([]types.Currency, error) {
	if m.ListCurrenciesFn != nil {
		return m.ListCurrenciesFn(ctx)
	}

	return nil, nil
}
