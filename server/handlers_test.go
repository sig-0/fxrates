package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sig-0/fxrates/storage/mock"

	"github.com/sig-0/fxrates/provider/currencies"
	"github.com/sig-0/fxrates/provider/ves"

	"github.com/sig-0/fxrates/storage/types"
)

func TestHandlers_RatesForPair(t *testing.T) {
	t.Parallel()

	t.Run("invalid base", func(t *testing.T) {
		t.Parallel()

		var called bool

		storage := &mock.Storage{
			RateAsOfFn: func(
				_ context.Context,
				_ *types.RateQuery,
				_ time.Time,
			) (*types.Page[*types.ExchangeRate], error) {
				called = true

				return nil, nil
			},
		}

		s := &Server{
			storage: storage,
			logger:  noopLogger,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/rates/US/VES", http.NoBody)
		req = withRouteParams(t, req, map[string]string{
			"base":   "US",
			"target": currencies.VES.String(),
		})

		w := httptest.NewRecorder()
		s.RatesForPair(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.False(t, called)
	})

	t.Run("storage error", func(t *testing.T) {
		t.Parallel()

		storage := &mock.Storage{
			RateAsOfFn: func(
				_ context.Context,
				_ *types.RateQuery,
				_ time.Time,
			) (*types.Page[*types.ExchangeRate], error) {
				return nil, errors.New("boom")
			},
		}

		s := &Server{
			storage: storage,
			logger:  noopLogger,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/rates/USD/VES", http.NoBody)
		req = withRouteParams(t, req, map[string]string{
			"base":   currencies.USD.String(),
			"target": currencies.VES.String(),
		})

		w := httptest.NewRecorder()
		s.RatesForPair(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var (
			capturedQuery *types.RateQuery
			capturedAsOf  time.Time
		)

		expectedAsOf := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)

		storage := &mock.Storage{
			RateAsOfFn: func(
				_ context.Context,
				query *types.RateQuery,
				asOf time.Time,
			) (*types.Page[*types.ExchangeRate], error) {
				capturedQuery = query
				capturedAsOf = asOf

				return &types.Page[*types.ExchangeRate]{
					Results: []*types.ExchangeRate{{
						Base:   currencies.USD,
						Target: currencies.VES,
						Rate:   42,
					}},
					Total: 1,
				}, nil
			},
		}

		s := &Server{
			storage: storage,
			logger:  noopLogger,
		}

		url := "/v1/rates/USD/VES?as_of=2026-01-10T00:00:00Z" +
			"&limit=200&offset=2&source=BCV&type=buy"
		req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
		req = withRouteParams(t, req, map[string]string{
			"base":   currencies.USD.String(),
			"target": currencies.VES.String(),
		})

		w := httptest.NewRecorder()
		s.RatesForPair(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var page types.Page[*types.ExchangeRate]

		require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
		require.Len(t, page.Results, 1)
		assert.Equal(t, int64(1), page.Total)

		require.NotNil(t, capturedQuery)
		assert.Equal(t, currencies.USD, capturedQuery.Base)

		require.NotNil(t, capturedQuery.Target)
		assert.Equal(t, currencies.VES, *capturedQuery.Target)

		require.NotNil(t, capturedQuery.Source)
		assert.Equal(t, ves.BCVSource, *capturedQuery.Source)
		require.NotNil(t, capturedQuery.RateType)

		assert.Equal(t, types.RateTypeBUY, *capturedQuery.RateType)
		assert.Equal(t, int32(200), capturedQuery.Limit)
		assert.Equal(t, int64(2), capturedQuery.Offset)
		assert.Equal(t, expectedAsOf, capturedAsOf)
	})
}

func TestHandlers_RatesForBase(t *testing.T) {
	t.Parallel()

	t.Run("storage error", func(t *testing.T) {
		t.Parallel()

		storage := &mock.Storage{
			RateAsOfFn: func(
				_ context.Context,
				_ *types.RateQuery,
				_ time.Time,
			) (*types.Page[*types.ExchangeRate], error) {
				return nil, errors.New("boom")
			},
		}

		s := &Server{
			storage: storage,
			logger:  noopLogger,
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/rates/USD", http.NoBody)
		req = withRouteParams(t, req, map[string]string{"base": currencies.USD.String()})

		w := httptest.NewRecorder()
		s.RatesForBase(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var (
			capturedQuery *types.RateQuery
			capturedAsOf  time.Time
		)

		expectedAsOf := time.Date(2026, time.January, 11, 0, 0, 0, 0, time.UTC)

		storage := &mock.Storage{
			RateAsOfFn: func(
				_ context.Context,
				query *types.RateQuery,
				asOf time.Time,
			) (*types.Page[*types.ExchangeRate], error) {
				capturedQuery = query
				capturedAsOf = asOf

				return &types.Page[*types.ExchangeRate]{
					Results: []*types.ExchangeRate{{
						Base:   currencies.USD,
						Target: currencies.VES,
						Rate:   50,
					}},
					Total: 1,
				}, nil
			},
		}

		s := &Server{
			storage: storage,
			logger:  noopLogger,
		}

		url := "/v1/rates/USD?as_of=2026-01-11T00:00:00Z&limit=50&offset=3&type=SELL"
		req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
		req = withRouteParams(t, req, map[string]string{"base": currencies.USD.String()})

		w := httptest.NewRecorder()
		s.RatesForBase(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var page types.Page[*types.ExchangeRate]

		require.NoError(t, json.NewDecoder(w.Body).Decode(&page))
		require.Len(t, page.Results, 1)
		assert.Equal(t, int64(1), page.Total)

		require.NotNil(t, capturedQuery)

		assert.Equal(t, currencies.USD, capturedQuery.Base)
		assert.Nil(t, capturedQuery.Target)

		require.NotNil(t, capturedQuery.RateType)
		assert.Equal(t, types.RateTypeSELL, *capturedQuery.RateType)

		assert.Equal(t, int32(50), capturedQuery.Limit)
		assert.Equal(t, int64(3), capturedQuery.Offset)

		assert.Equal(t, expectedAsOf, capturedAsOf)
	})
}

func TestHandlers_ListEndpoints(t *testing.T) {
	t.Parallel()

	testTable := []struct {
		name     string
		path     string
		handler  func(*Server, http.ResponseWriter, *http.Request)
		expected []string
	}{
		{
			name: "sources",
			path: "/v1/sources",
			handler: func(s *Server, w http.ResponseWriter, r *http.Request) {
				s.Sources(w, r)
			},
			expected: []string{ves.BCVSource.String(), ves.BinanceP2PSource.String()},
		},
		{
			name: "currencies",
			path: "/v1/currencies",
			handler: func(s *Server, w http.ResponseWriter, r *http.Request) {
				s.Currencies(w, r)
			},
			expected: []string{currencies.USD.String(), currencies.VES.String()},
		},
	}

	for _, testCase := range testTable {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			t.Run("storage error", func(t *testing.T) {
				t.Parallel()

				s := &Server{
					storage: listStorage(t, testCase.name, nil, errors.New("boom")),
					logger:  noopLogger,
				}

				req := httptest.NewRequest(http.MethodGet, testCase.path, http.NoBody)
				w := httptest.NewRecorder()

				testCase.handler(s, w, req)

				assert.Equal(t, http.StatusInternalServerError, w.Code)
			})

			t.Run("success", func(t *testing.T) {
				t.Parallel()

				s := &Server{
					storage: listStorage(t, testCase.name, testCase.expected, nil),
					logger:  noopLogger,
				}

				req := httptest.NewRequest(http.MethodGet, testCase.path, http.NoBody)
				w := httptest.NewRecorder()

				testCase.handler(s, w, req)

				require.Equal(t, http.StatusOK, w.Code)
				assert.Equal(t, testCase.expected, decodeListResults(t, w))
			})
		})
	}
}

func TestUtils_ParseAsOf(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		expected := time.Date(2026, time.January, 12, 0, 0, 0, 0, time.UTC)

		value, err := parseAsOf("2026-01-12T00:00:00Z")

		require.NoError(t, err)
		assert.Equal(t, expected, value)
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		_, err := parseAsOf("nope")

		assert.Error(t, err)
	})
}

func TestUtils_ParseLimitOffset(t *testing.T) {
	t.Parallel()

	t.Run("defaults", func(t *testing.T) {
		t.Parallel()

		limit, offset, err := parseLimitOffset("", "")

		require.NoError(t, err)
		assert.Equal(t, int32(100), limit)
		assert.Equal(t, int64(0), offset)
	})

	t.Run("clamps limit", func(t *testing.T) {
		t.Parallel()

		limit, offset, err := parseLimitOffset("999", "5")

		require.NoError(t, err)
		assert.Equal(t, int32(500), limit)
		assert.Equal(t, int64(5), offset)
	})

	t.Run("invalid limit", func(t *testing.T) {
		t.Parallel()

		_, _, err := parseLimitOffset("nope", "0")

		assert.ErrorIs(t, err, errInvalidLimit)
	})

	t.Run("invalid offset", func(t *testing.T) {
		t.Parallel()

		_, _, err := parseLimitOffset("10", "nope")

		assert.ErrorIs(t, err, errInvalidOffset)
	})
}

func TestUtils_ParseSourceAndType(t *testing.T) {
	t.Parallel()

	t.Run("valid type", func(t *testing.T) {
		t.Parallel()

		source, rateType, err := parseSourceAndType(
			ves.BCVSource.String(),
			"buy",
		)

		require.NoError(t, err)
		require.NotNil(t, source)
		require.NotNil(t, rateType)

		assert.Equal(t, ves.BCVSource, *source)
		assert.Equal(t, types.RateTypeBUY, *rateType)
	})

	t.Run("invalid type", func(t *testing.T) {
		t.Parallel()

		_, _, err := parseSourceAndType("", "nope")

		assert.ErrorIs(t, err, errInvalidType)
	})
}

func TestUtils_ParseCurrencySymbol(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		value, err := parseCurrencySymbol(currencies.USD.String())

		require.NoError(t, err)
		assert.Equal(t, currencies.USD, value)
	})

	t.Run("valid length 4", func(t *testing.T) {
		t.Parallel()

		value, err := parseCurrencySymbol(currencies.USDT.String())

		require.NoError(t, err)
		assert.Equal(t, currencies.USDT, value)
	})

	t.Run("invalid length", func(t *testing.T) {
		t.Parallel()

		_, err := parseCurrencySymbol("usdtt")

		assert.Error(t, err)
	})

	t.Run("invalid chars", func(t *testing.T) {
		t.Parallel()

		_, err := parseCurrencySymbol("US$")

		assert.Error(t, err)
	})
}

func withRouteParams(t *testing.T, req *http.Request, params map[string]string) *http.Request {
	t.Helper()

	rctx := chi.NewRouteContext()

	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}

	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func listStorage(t *testing.T, kind string, results []string, err error) *mock.Storage {
	t.Helper()

	switch kind {
	case "sources":
		return &mock.Storage{
			ListSourcesFn: func(_ context.Context) ([]types.Source, error) {
				if err != nil {
					return nil, err
				}

				return toItems[types.Source](t, results), nil
			},
		}
	case "currencies":
		return &mock.Storage{
			ListCurrenciesFn: func(_ context.Context) ([]types.Currency, error) {
				if err != nil {
					return nil, err
				}

				return toItems[types.Currency](t, results), nil
			},
		}
	default:
		return &mock.Storage{}
	}
}

func toItems[T ~string](t *testing.T, results []string) []T {
	t.Helper()

	items := make([]T, 0, len(results))
	for _, value := range results {
		items = append(items, T(value))
	}

	return items
}

func decodeListResults(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()

	var resp struct {
		Results []string `json:"results"`
	}

	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

	return resp.Results
}
