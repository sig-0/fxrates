package dolarapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

const (
	dolaresURL  = "https://dolarapi.com/v1/dolares"
	monedaUSD   = "USD"
	casaBlue    = "blue"
	casaBolsa   = "bolsa"
	casaCCL     = "contadoconliqui"
	casaTarjeta = "tarjeta"
)

var requiredCasas = map[string]types.Source{
	casaBlue:    types.SourceDolarAPIBlue,
	casaBolsa:   types.SourceDolarAPIMEP,
	casaCCL:     types.SourceDolarAPICCL,
	casaTarjeta: types.SourceDolarAPITarjeta,
}

//nolint:tagliatelle // External API field names.
type entry struct {
	Moneda             string  `json:"moneda"`
	Casa               string  `json:"casa"`
	Nombre             string  `json:"nombre"`
	FechaActualizacion string  `json:"fechaActualizacion"`
	Compra             float64 `json:"compra"`
	Venta              float64 `json:"venta"`
}

type Provider struct {
	client *http.Client
	url    string
}

func New(timeout time.Duration) *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: dolaresURL,
	}
}

func (p *Provider) Name() string {
	return "DolarAPI (ARS parallel)"
}

func (p *Provider) Interval() time.Duration {
	return time.Minute * 10
}

func (p *Provider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("unable to create GET request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to execute GET request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code received: %d", resp.StatusCode)
	}

	var entries []entry

	if err = json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	fetchTime := time.Now().UTC()
	seen := make(map[string]struct{}, len(requiredCasas))
	rates := make([]*types.ExchangeRate, 0, len(requiredCasas)*3)

	for _, e := range entries {
		if e.Moneda != monedaUSD {
			continue
		}

		source, required := requiredCasas[e.Casa]
		if !required {
			continue
		}

		if _, dup := seen[e.Casa]; dup {
			return nil, fmt.Errorf("DolarAPI returned duplicate casa %q", e.Casa)
		}

		seen[e.Casa] = struct{}{}

		if e.Compra <= 0 || e.Venta <= 0 {
			return nil, fmt.Errorf(
				"DolarAPI casa %q rate is non-positive: compra=%v venta=%v",
				e.Casa, e.Compra, e.Venta,
			)
		}

		asOf, err := time.Parse(time.RFC3339Nano, e.FechaActualizacion)
		if err != nil {
			return nil, fmt.Errorf(
				"unable to parse fechaActualizacion %q for casa %q: %w",
				e.FechaActualizacion, e.Casa, err,
			)
		}

		midRate := math.Round((e.Compra+e.Venta)/2*1e4) / 1e4

		rates = append(
			rates,
			&types.ExchangeRate{
				AsOf:      asOf.UTC(),
				FetchedAt: fetchTime,
				Base:      types.CurrencyUSD,
				Target:    types.CurrencyARS,
				RateType:  types.RateTypeBUY,
				Source:    source,
				Rate:      math.Round(e.Compra*1e4) / 1e4,
			},
			&types.ExchangeRate{
				AsOf:      asOf.UTC(),
				FetchedAt: fetchTime,
				Base:      types.CurrencyUSD,
				Target:    types.CurrencyARS,
				RateType:  types.RateTypeSELL,
				Source:    source,
				Rate:      math.Round(e.Venta*1e4) / 1e4,
			},
			&types.ExchangeRate{
				AsOf:      asOf.UTC(),
				FetchedAt: fetchTime,
				Base:      types.CurrencyUSD,
				Target:    types.CurrencyARS,
				RateType:  types.RateTypeMID,
				Source:    source,
				Rate:      midRate,
			},
		)
	}

	if len(seen) != len(requiredCasas) {
		missing := make([]string, 0, len(requiredCasas))

		for casa := range requiredCasas {
			if _, ok := seen[casa]; !ok {
				missing = append(missing, casa)
			}
		}

		sort.Strings(missing)

		return nil, fmt.Errorf("DolarAPI missing required casas: %v", missing)
	}

	return rates, nil
}
