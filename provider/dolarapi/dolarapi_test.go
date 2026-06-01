package dolarapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sig-0/fxrates/storage/types"
)

func newEntry(casa string, compra, venta float64, fechaActualizacion string) entry {
	return entry{
		Moneda:             monedaUSD,
		Casa:               casa,
		Nombre:             casa,
		Compra:             compra,
		Venta:              venta,
		FechaActualizacion: fechaActualizacion,
	}
}

func defaultDolarAPIPayload() []entry {
	return []entry{
		newEntry(casaBlue, 1180.0, 1200.0, "2026-05-31T14:08:30.500Z"),
		newEntry(casaBolsa, 1145.0, 1148.0, "2026-05-31T14:09:15.000Z"),
		newEntry(casaCCL, 1170.0, 1175.0, "2026-05-31T14:09:45.000Z"),
		newEntry(casaTarjeta, 1640.0, 1640.0, "2026-05-31T14:05:00.000Z"),
		// Casa the provider must silently skip.
		newEntry("oficial", 985.0, 1025.0, "2026-05-31T14:05:00.123Z"),
		// Forward-compatibility: an unknown casa is silently ignored.
		newEntry("future_casa", 9999.0, 9999.0, "2026-05-31T14:10:00.000Z"),
	}
}

func encodeDolarAPIPayload(t *testing.T, payload []entry) string {
	t.Helper()

	out, err := json.Marshal(payload)
	require.NoError(t, err)

	return string(out)
}

func newDolarAPIServer(t *testing.T, status int, body string) *httptest.Server {
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

func newDolarAPIProviderWith(srv *httptest.Server) *Provider {
	p := New(time.Second * 5)
	p.url = srv.URL

	return p
}

func TestDolarAPI_Fetch_HappyPath(t *testing.T) {
	t.Parallel()

	srv := newDolarAPIServer(t, http.StatusOK, encodeDolarAPIPayload(t, defaultDolarAPIPayload()))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	rates, err := newDolarAPIProviderWith(srv).Fetch(ctx)
	require.NoError(t, err)
	// 4 required casas x 3 rate types = 12 rows; ignored casas dropped.
	require.Len(t, rates, 12)

	bySrc := make(map[types.Source]map[types.RateType]*types.ExchangeRate, 4)

	for _, r := range rates {
		assert.Equal(t, types.CurrencyUSD, r.Base)
		assert.Equal(t, types.CurrencyARS, r.Target)

		if _, ok := bySrc[r.Source]; !ok {
			bySrc[r.Source] = make(map[types.RateType]*types.ExchangeRate, 3)
		}

		bySrc[r.Source][r.RateType] = r
	}

	want := map[types.Source]struct {
		asOf           time.Time
		buy, sell, mid float64
	}{
		types.SourceDolarAPIBlue: {
			buy: 1180.0, sell: 1200.0, mid: 1190.0,
			asOf: time.Date(2026, time.May, 31, 14, 8, 30, 500_000_000, time.UTC),
		},
		types.SourceDolarAPIMEP: {
			buy: 1145.0, sell: 1148.0, mid: 1146.5,
			asOf: time.Date(2026, time.May, 31, 14, 9, 15, 0, time.UTC),
		},
		types.SourceDolarAPICCL: {
			buy: 1170.0, sell: 1175.0, mid: 1172.5,
			asOf: time.Date(2026, time.May, 31, 14, 9, 45, 0, time.UTC),
		},
		types.SourceDolarAPITarjeta: {
			buy: 1640.0, sell: 1640.0, mid: 1640.0,
			asOf: time.Date(2026, time.May, 31, 14, 5, 0, 0, time.UTC),
		},
	}

	for src, w := range want {
		entries, ok := bySrc[src]
		require.True(t, ok, "missing source %s", src)
		require.Len(t, entries, 3, "source %s should have 3 rate types", src)

		assert.InEpsilon(t, w.buy, entries[types.RateTypeBUY].Rate, 1e-9, src)
		assert.InEpsilon(t, w.sell, entries[types.RateTypeSELL].Rate, 1e-9, src)
		assert.InEpsilon(t, w.mid, entries[types.RateTypeMID].Rate, 1e-9, src)
		assert.Equal(t, w.asOf, entries[types.RateTypeBUY].AsOf)
	}
}

func TestDolarAPI_Fetch_ErrorCases(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		mutate  func([]entry) []entry
		name    string
		wantSub string
	}{
		{
			name: "missing blue casa",
			mutate: func(p []entry) []entry {
				out := make([]entry, 0, len(p))

				for _, e := range p {
					if e.Casa != casaBlue {
						out = append(out, e)
					}
				}

				return out
			},
			wantSub: casaBlue,
		},
		{
			name: "duplicate blue casa",
			mutate: func(p []entry) []entry {
				return append(p, newEntry(casaBlue, 1, 2, "2026-05-31T14:08:30.500Z"))
			},
			wantSub: `duplicate casa "blue"`,
		},
		{
			name: "malformed fechaActualizacion",
			mutate: func(p []entry) []entry {
				for i := range p {
					if p[i].Casa == casaBlue {
						p[i].FechaActualizacion = "not-a-date"
					}
				}

				return p
			},
			wantSub: "fechaActualizacion",
		},
		{
			name: "bolsa compra is zero",
			mutate: func(p []entry) []entry {
				for i := range p {
					if p[i].Casa == casaBolsa {
						p[i].Compra = 0.0
					}
				}

				return p
			},
			wantSub: "non-positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newDolarAPIServer(t, http.StatusOK, encodeDolarAPIPayload(t, tc.mutate(defaultDolarAPIPayload())))
			t.Cleanup(srv.Close)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			t.Cleanup(cancel)

			_, err := newDolarAPIProviderWith(srv).Fetch(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSub)
		})
	}
}
