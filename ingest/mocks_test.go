package ingest

import (
	"context"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

type (
	nameDelegate     func() string
	intervalDelegate func() time.Duration
	fetchDelegate    func(context.Context) ([]*types.ExchangeRate, error)
)

type mockProvider struct {
	nameFn     nameDelegate
	intervalFn intervalDelegate
	fetchFn    fetchDelegate
}

func (m *mockProvider) Name() string {
	if m.nameFn != nil {
		return m.nameFn()
	}

	return ""
}

func (m *mockProvider) Interval() time.Duration {
	if m.intervalFn != nil {
		return m.intervalFn()
	}

	return 0
}

func (m *mockProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	if m.fetchFn != nil {
		return m.fetchFn(ctx)
	}

	return nil, nil
}
