package clp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/sig-0/fxrates/provider/currencies"
	"github.com/sig-0/fxrates/storage/types"
)

var BCCHSource types.Source = "BCCH"

const findicURL = "https://findic.cl/api/dolar"

// findicResponse is the top-level JSON response from findic.cl.
type findicResponse struct {
	Serie []findicEntry `json:"serie"`
}

// findicEntry represents a single rate entry.
type findicEntry struct {
	Fecha string  `json:"fecha"`
	Valor float64 `json:"valor"`
}

// FindicProvider fetches USD/CLP "Dólar Observado" from findic.cl.
type FindicProvider struct {
	client *http.Client
	url    string
}

// NewFindicProvider creates a new instance of the Findic provider.
func NewFindicProvider(timeout time.Duration) *FindicProvider {
	return &FindicProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: findicURL,
	}
}

func (p *FindicProvider) Name() string {
	return "Findic (CLP)"
}

func (p *FindicProvider) Interval() time.Duration {
	return time.Hour * 3
}

func (p *FindicProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
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

	var apiResp findicResponse

	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	if len(apiResp.Serie) == 0 {
		return nil, fmt.Errorf("no rate entries returned")
	}

	entry := apiResp.Serie[0]

	asOf, err := time.Parse("2006-01-02", entry.Fecha)
	if err != nil {
		return nil, fmt.Errorf("unable to parse date %q: %w", entry.Fecha, err)
	}

	return []*types.ExchangeRate{
		{
			AsOf:      asOf.UTC(),
			FetchedAt: time.Now().UTC(),
			Base:      currencies.USD,
			Target:    currencies.CLP,
			RateType:  types.RateTypeMID,
			Source:    BCCHSource,
			Rate:      math.Round(entry.Valor*1e4) / 1e4,
		},
	}, nil
}
