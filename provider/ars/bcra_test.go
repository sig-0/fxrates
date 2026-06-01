package ars

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sig-0/fxrates/storage/types"
)

const bcraHappyBody = `{
  "status": 200,
  "results": {
    "fecha": "2026-05-30",
    "detalle": [
      {"codigoMoneda": "EUR", "tipoCotizacion": 1334.25},
      {"codigoMoneda": "USD", "tipoCotizacion": 1235.5}
    ]
  }
}`

func newBCRAServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if status != 0 {
			w.WriteHeader(status)
		}

		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	}))
}

func newBCRAProviderWith(srv *httptest.Server) *BCRAProvider {
	p := NewBCRAProvider(time.Second * 5)
	p.url = srv.URL

	return p
}

func TestBCRA_Fetch_HappyPath(t *testing.T) {
	t.Parallel()

	srv := newBCRAServer(t, http.StatusOK, bcraHappyBody)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	rates, err := newBCRAProviderWith(srv).Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, rates, 1)

	rate := rates[0]
	assert.Equal(t, types.CurrencyUSD, rate.Base)
	assert.Equal(t, types.CurrencyARS, rate.Target)
	assert.Equal(t, types.RateTypeMID, rate.RateType)
	assert.Equal(t, types.SourceBCRA, rate.Source)
	assert.InEpsilon(t, 1235.5, rate.Rate, 1e-9)

	loc, err := time.LoadLocation(bcraTimezoneName)
	require.NoError(t, err)

	expected := time.Date(2026, time.May, 30, 0, 0, 0, 0, loc).UTC()
	assert.Equal(t, expected, rate.AsOf)
}

func TestBCRA_Fetch_ErrorCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		body   string
		status int
	}{
		{
			name:   "application status 500",
			status: http.StatusOK,
			body:   `{"status":500,"results":{"fecha":"2026-05-30","detalle":[{"codigoMoneda":"USD","tipoCotizacion":1000.0}]}}`,
		},
		{
			name:   "application status 400 empty",
			status: http.StatusOK,
			body:   `{"status":400}`,
		},
		{
			name:   "missing results key",
			status: http.StatusOK,
			body:   `{"status":200}`,
		},
		{
			name:   "empty fecha",
			status: http.StatusOK,
			body:   `{"status":200,"results":{"fecha":"","detalle":[{"codigoMoneda":"USD","tipoCotizacion":1000.0}]}}`,
		},
		{
			name:   "malformed fecha",
			status: http.StatusOK,
			body:   `{"status":200,"results":{"fecha":"2026/05/30","detalle":[{"codigoMoneda":"USD","tipoCotizacion":1000.0}]}}`,
		},
		{
			name:   "empty detalle",
			status: http.StatusOK,
			body:   `{"status":200,"results":{"fecha":"2026-05-30","detalle":[]}}`,
		},
		{
			name:   "no USD entry",
			status: http.StatusOK,
			body:   `{"status":200,"results":{"fecha":"2026-05-30","detalle":[{"codigoMoneda":"EUR","tipoCotizacion":1334.0}]}}`,
		},
		{
			name:   "USD rate zero",
			status: http.StatusOK,
			body:   `{"status":200,"results":{"fecha":"2026-05-30","detalle":[{"codigoMoneda":"USD","tipoCotizacion":0}]}}`,
		},
		{
			name:   "USD rate negative",
			status: http.StatusOK,
			body:   `{"status":200,"results":{"fecha":"2026-05-30","detalle":[{"codigoMoneda":"USD","tipoCotizacion":-1}]}}`,
		},
		{
			name:   "malformed JSON",
			status: http.StatusOK,
			body:   `not-json`,
		},
		{
			name:   "non-2xx HTTP",
			status: http.StatusInternalServerError,
			body:   `{}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newBCRAServer(t, tc.status, tc.body)
			t.Cleanup(srv.Close)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			t.Cleanup(cancel)

			_, err := newBCRAProviderWith(srv).Fetch(ctx)
			assert.Error(t, err)
		})
	}
}
