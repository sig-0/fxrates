package ars

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sig-0/fxrates/storage/types"
)

// bnaPage builds the minimum HTML the provider needs. Each test starts from
// defaultBNAPage() and mutates one field to express a negative scenario.
type bnaPage struct {
	BilletesAnchor    string
	BilletesUSDLabel  string
	BilletesCompra    string
	BilletesVenta     string
	BilletesDate      string
	BilletesHora      string
	DivisasUSDLabel   string
	BilletesSectionID string
	OmitBilletesHora  bool
	OmitBilletesDate  bool
}

func defaultBNAPage() bnaPage {
	return bnaPage{
		BilletesAnchor:    `<a href="#billetes" data-toggle="tab">Cotización Billetes</a>`,
		BilletesSectionID: "billetes",
		BilletesUSDLabel:  bnaDolarRowLabel,
		BilletesCompra:    "1.380,00",
		BilletesVenta:     "1.430,00",
		BilletesDate:      "29/5/2026",
		BilletesHora:      "17:00",
		DivisasUSDLabel:   bnaDolarRowLabel,
	}
}

func (p bnaPage) render() string {
	var billetesHead string
	if !p.OmitBilletesDate {
		billetesHead = fmt.Sprintf(`<th class="fechaCot">%s</th>`, p.BilletesDate)
	}

	var billetesHora string
	if !p.OmitBilletesHora {
		billetesHora = fmt.Sprintf(`<div class="legal">Hora Actualización: %s</div>`, p.BilletesHora)
	}

	return fmt.Sprintf(
		`<!doctype html>
<html><body>
<ul class="nav nav-tabs">
  <li>%s</li>
  <li><a href="#divisas" data-toggle="tab">Cotización Divisas</a></li>
</ul>
<div class="tab-content">
  <div id="%s">
    <table>
      <thead><tr>%s<th>Compra</th><th>Venta</th></tr></thead>
      <tbody>
        <tr><td>%s</td><td>%s</td><td>%s</td></tr>
      </tbody>
    </table>
    %s
  </div>
  <div id="divisas">
    <table>
      <tbody>
        <tr><td>%s</td><td>1399.0000</td><td>1408.0000</td></tr>
      </tbody>
    </table>
  </div>
</div>
</body></html>`,
		p.BilletesAnchor,
		p.BilletesSectionID,
		billetesHead,
		p.BilletesUSDLabel, p.BilletesCompra, p.BilletesVenta,
		billetesHora,
		p.DivisasUSDLabel,
	)
}

func newBNAServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		_, err := w.Write([]byte(body))
		require.NoError(t, err)
	}))
}

func newBNAProviderWith(srv *httptest.Server) *BNAProvider {
	p := NewBNAProvider(time.Second * 5)
	p.url = srv.URL

	return p
}

func TestBNAProvider_Fetch(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		srv := newBNAServer(t, defaultBNAPage().render())
		t.Cleanup(srv.Close)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		t.Cleanup(cancel)

		rates, err := newBNAProviderWith(srv).Fetch(ctx)
		require.NoError(t, err)
		require.Len(t, rates, 3)

		for _, r := range rates {
			assert.Equal(t, types.CurrencyUSD, r.Base)
			assert.Equal(t, types.CurrencyARS, r.Target)
			assert.Equal(t, types.SourceBNA, r.Source)
		}

		assert.Equal(t, types.RateTypeBUY, rates[0].RateType)
		assert.InEpsilon(t, 1380.0, rates[0].Rate, 1e-9)

		assert.Equal(t, types.RateTypeSELL, rates[1].RateType)
		assert.InEpsilon(t, 1430.0, rates[1].Rate, 1e-9)

		assert.Equal(t, types.RateTypeMID, rates[2].RateType)
		assert.InEpsilon(t, 1405.0, rates[2].Rate, 1e-9)

		loc, err := time.LoadLocation(bcraTimezoneName)
		require.NoError(t, err)

		expected := time.Date(2026, time.May, 29, 17, 0, 0, 0, loc).UTC()
		assert.Equal(t, expected, rates[0].AsOf)
		assert.Equal(t, expected, rates[1].AsOf)
		assert.Equal(t, expected, rates[2].AsOf)
	})

	errorCases := []struct {
		mutate  func(*bnaPage)
		name    string
		wantSub string
	}{
		{
			name:    "billetes anchor missing",
			mutate:  func(p *bnaPage) { p.BilletesAnchor = "" },
			wantSub: "section header missing",
		},
		{
			name:    "billetes section div missing",
			mutate:  func(p *bnaPage) { p.BilletesSectionID = "billetes-disabled" },
			wantSub: "section (#billetes) missing",
		},
		{
			name:    "dolar u.s.a row missing in billetes",
			mutate:  func(p *bnaPage) { p.BilletesUSDLabel = "Dolar XXX" },
			wantSub: bnaDolarRowLabel,
		},
		{
			name:    "date node missing in billetes",
			mutate:  func(p *bnaPage) { p.OmitBilletesDate = true },
			wantSub: "date node missing",
		},
		{
			name:    "hora actualización missing",
			mutate:  func(p *bnaPage) { p.OmitBilletesHora = true },
			wantSub: "Hora Actualización missing",
		},
		{
			name:    "usd row only in divisas table (billetes empty)",
			mutate:  func(p *bnaPage) { p.BilletesUSDLabel = "Dolar Estadounidense" },
			wantSub: bnaDolarRowLabel,
		},
		{
			name:    "billetes compra is zero",
			mutate:  func(p *bnaPage) { p.BilletesCompra = "0,00" },
			wantSub: "non-positive",
		},
		{
			name: "stray wrapper div with billetes text but no anchor",
			mutate: func(p *bnaPage) {
				p.BilletesAnchor = `<div class="random">Cotización Billetes</div>`
			},
			wantSub: "section header missing",
		},
		{
			name:    "malformed date",
			mutate:  func(p *bnaPage) { p.BilletesDate = "not-a-date" },
			wantSub: "BNA date",
		},
		{
			name:    "malformed hour",
			mutate:  func(p *bnaPage) { p.BilletesHora = "nope" },
			wantSub: "BNA hour",
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			page := defaultBNAPage()
			tc.mutate(&page)

			srv := newBNAServer(t, page.render())
			t.Cleanup(srv.Close)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			t.Cleanup(cancel)

			_, err := newBNAProviderWith(srv).Fetch(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantSub)
		})
	}
}

func TestParseARSNumber(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "thousands and decimals", input: "1.380,00", want: 1380.0},
		{name: "small decimal", input: "5,25", want: 5.25},
		{name: "integer", input: "42", want: 42.0},
		{name: "empty", input: "", wantErr: true},
		{name: "alpha", input: "abc", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseARSNumber(tc.input)
			if tc.wantErr {
				assert.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.InEpsilon(t, tc.want, got, 1e-9)
		})
	}
}
