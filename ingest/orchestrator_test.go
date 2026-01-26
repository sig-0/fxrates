package ingest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sig-0/fxrates/storage/mock"

	"github.com/sig-0/fxrates/provider/currencies"

	"github.com/sig-0/fxrates/storage/types"
)

const testProviderName = "test-provider"

func TestOrchestrator_New(t *testing.T) {
	t.Parallel()

	t.Run("default orchestrator", func(t *testing.T) {
		t.Parallel()

		o := New(&mock.Storage{})

		require.NotNil(t, o)

		assert.NotNil(t, o.storage)
		assert.NotNil(t, o.logger)
		assert.Equal(t, time.Second, o.queryInterval)
	})

	t.Run("query interval", func(t *testing.T) {
		t.Parallel()

		o := New(&mock.Storage{}, WithQueryInterval(time.Minute))

		require.NotNil(t, o)
		assert.Equal(t, time.Minute, o.queryInterval)
	})
}

func TestOrchestrator_Register(t *testing.T) {
	t.Parallel()

	t.Run("nil provider", func(t *testing.T) {
		t.Parallel()

		o := New(&mock.Storage{})

		assert.ErrorIs(t, o.Register(nil), errInvalidProvider)
	})

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		var (
			o = New(&mock.Storage{})

			provider = &mockProvider{
				nameFn: func() string {
					return ""
				},
				intervalFn: func() time.Duration {
					return time.Hour
				},
			}
		)

		assert.ErrorIs(t, o.Register(provider), errInvalidProvider)
	})

	t.Run("zero interval", func(t *testing.T) {
		t.Parallel()

		var (
			o = New(&mock.Storage{})

			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return 0
				},
			}
		)

		assert.ErrorIs(t, o.Register(provider), errInvalidInterval)
	})

	t.Run("negative interval", func(t *testing.T) {
		t.Parallel()

		var (
			o = New(&mock.Storage{})

			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return -time.Hour
				},
			}
		)

		assert.ErrorIs(t, o.Register(provider), errInvalidInterval)
	})

	t.Run("valid provider", func(t *testing.T) {
		t.Parallel()

		var (
			o = New(&mock.Storage{})

			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return time.Hour
				},
			}
		)

		require.NoError(t, o.Register(provider))

		// Verify provider was registered
		var count int

		o.registeredProviders.Range(
			func(_, _ any) bool {
				count++

				return true
			},
		)

		assert.Equal(t, 1, count)
	})

	t.Run("schedule provider", func(t *testing.T) {
		t.Parallel()

		var (
			o = New(&mock.Storage{})

			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return time.Hour
				},
			}
		)

		require.NoError(t, o.Register(provider))
		assert.Equal(t, 1, o.q.Len())

		// The scheduled time should be in the past or now (immediate)
		scheduled := o.q.Index(0)
		assert.True(t, scheduled.at.Before(time.Now().Add(time.Second)))
	})
}

func TestOrchestrator_Start(t *testing.T) {
	t.Parallel()

	t.Run("ctx canceled", func(t *testing.T) {
		t.Parallel()

		var (
			o     = New(&mock.Storage{}, WithQueryInterval(time.Millisecond*10))
			errCh = make(chan error, 1)
		)

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			errCh <- o.Start(ctx)
		}()

		cancel()

		select {
		case err := <-errCh:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("orchestrator did not shut down in time")
		}
	})

	t.Run("provider fetch executed", func(t *testing.T) {
		t.Parallel()

		var (
			savedRate *types.ExchangeRate
			saveDone  = make(chan struct{})

			expectedRate = &types.ExchangeRate{
				Base:     currencies.USD,
				Target:   currencies.VES,
				Rate:     100.0,
				RateType: types.RateTypeMID,
				Source:   "test",
			}

			storage = &mock.Storage{
				SaveExchangeRateFn: func(_ context.Context, rate *types.ExchangeRate) error {
					savedRate = rate

					close(saveDone)

					return nil
				},
			}

			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return time.Hour
				},
				fetchFn: func(_ context.Context) ([]*types.ExchangeRate, error) {
					return []*types.ExchangeRate{
						expectedRate,
					}, nil
				},
			}
		)

		var (
			o     = New(storage, WithQueryInterval(time.Millisecond*10))
			errCh = make(chan error, 1)
		)

		require.NoError(t, o.Register(provider))

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			errCh <- o.Start(ctx)
		}()

		select {
		case <-saveDone:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for rate to be saved")
		}

		cancel()
		require.NoError(t, <-errCh)

		require.NotNil(t, savedRate)
		assert.Equal(t, expectedRate.Base, savedRate.Base)
		assert.Equal(t, expectedRate.Target, savedRate.Target)
		assert.Equal(t, expectedRate.Rate, savedRate.Rate)
	})

	t.Run("reschedule provider (success)", func(t *testing.T) {
		t.Parallel()

		var (
			fetchCount atomic.Int32
			fetchDone  = make(chan struct{})
		)

		var (
			storage = &mock.Storage{
				SaveExchangeRateFn: func(_ context.Context, _ *types.ExchangeRate) error {
					return nil
				},
			}

			o = New(storage, WithQueryInterval(time.Millisecond*10))

			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return time.Millisecond * 50
				},
				fetchFn: func(_ context.Context) ([]*types.ExchangeRate, error) {
					if fetchCount.Add(1) == 2 {
						close(fetchDone)
					}

					return []*types.ExchangeRate{{
						Base:   currencies.USD,
						Target: currencies.VES,
						Rate:   100.0,
					}}, nil
				},
			}
			errCh = make(chan error, 1)
		)

		require.NoError(t, o.Register(provider))

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			errCh <- o.Start(ctx)
		}()

		select {
		case <-fetchDone:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for reschedule")
		}

		cancel()
		require.NoError(t, <-errCh)

		assert.GreaterOrEqual(t, fetchCount.Load(), int32(2))
	})

	t.Run("retries on fetch error", func(t *testing.T) {
		t.Parallel()

		var (
			fetchCount atomic.Int32
			retryDone  = make(chan struct{})
		)

		var (
			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return time.Hour
				},
				fetchFn: func(_ context.Context) ([]*types.ExchangeRate, error) {
					if fetchCount.Add(1) == 2 {
						close(retryDone)
					}

					return nil, errors.New("fetch error")
				},
			}

			o = New(&mock.Storage{}, WithQueryInterval(time.Millisecond*10))

			errCh = make(chan error, 1)
		)

		require.NoError(t, o.Register(provider))

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			errCh <- o.Start(ctx)
		}()

		select {
		case <-retryDone:
			// Success
		case <-time.After(time.Second * 15):
			t.Fatal("timeout waiting for retry")
		}

		cancel()
		require.NoError(t, <-errCh)

		assert.GreaterOrEqual(t, fetchCount.Load(), int32(2))
	})

	t.Run("multiple providers", func(t *testing.T) {
		t.Parallel()

		var (
			savedRates sync.Map
			saveCount  atomic.Int32
			allSaved   = make(chan struct{})
			errCh      = make(chan error, 1)

			storage = &mock.Storage{
				SaveExchangeRateFn: func(_ context.Context, rate *types.ExchangeRate) error {
					savedRates.Store(rate.Source.String(), rate)

					if saveCount.Add(1) == 2 {
						close(allSaved)
					}

					return nil
				},
			}
			providers = []*mockProvider{
				{
					nameFn: func() string {
						return "provider-1"
					},
					intervalFn: func() time.Duration {
						return time.Hour
					},
					fetchFn: func(_ context.Context) ([]*types.ExchangeRate, error) {
						return []*types.ExchangeRate{{
							Base:   currencies.USD,
							Target: currencies.VES,
							Rate:   100.0,
							Source: "source-1",
						}}, nil
					},
				},
				{
					nameFn: func() string {
						return "provider-2"
					},
					intervalFn: func() time.Duration {
						return time.Hour
					},
					fetchFn: func(_ context.Context) ([]*types.ExchangeRate, error) {
						return []*types.ExchangeRate{{
							Base:   currencies.EUR,
							Target: currencies.VES,
							Rate:   110.0,
							Source: "source-2",
						}}, nil
					},
				},
			}

			o = New(storage, WithQueryInterval(time.Millisecond*10))
		)

		for _, p := range providers {
			require.NoError(t, o.Register(p))
		}

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			errCh <- o.Start(ctx)
		}()

		select {
		case <-allSaved:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for providers")
		}

		cancel()
		require.NoError(t, <-errCh)

		_, ok1 := savedRates.Load("source-1")
		_, ok2 := savedRates.Load("source-2")

		assert.True(t, ok1, "source-1 should be saved")
		assert.True(t, ok2, "source-2 should be saved")
	})

	t.Run("storage save error", func(t *testing.T) {
		t.Parallel()

		var (
			saveAttempts atomic.Int32
			savesDone    = make(chan struct{})
			errCh        = make(chan error, 1)

			storage = &mock.Storage{
				SaveExchangeRateFn: func(_ context.Context, _ *types.ExchangeRate) error {
					if saveAttempts.Add(1) == 2 {
						close(savesDone)
					}

					return errors.New("storage error")
				},
			}
			provider = &mockProvider{
				nameFn: func() string {
					return testProviderName
				},
				intervalFn: func() time.Duration {
					return time.Millisecond * 50
				},
				fetchFn: func(_ context.Context) ([]*types.ExchangeRate, error) {
					return []*types.ExchangeRate{{
						Base:   currencies.USD,
						Target: currencies.VES,
						Rate:   100.0,
					}}, nil
				},
			}

			o = New(storage, WithQueryInterval(time.Millisecond*10))
		)

		require.NoError(t, o.Register(provider))

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			errCh <- o.Start(ctx)
		}()

		select {
		case <-savesDone:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for save attempts")
		}

		cancel()
		require.NoError(t, <-errCh)
	})
}
