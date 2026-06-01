package ars

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

const (
	bcraURL          = "https://api.bcra.gob.ar/estadisticascambiarias/v1.0/Cotizaciones"
	bcraUSDCode      = "USD"
	bcraDateLayout   = "2006-01-02"
	bcraStatusOK     = 200
	bcraTimezoneName = "America/Argentina/Buenos_Aires"
)

type bcraResponse struct {
	Results *bcraSnapshot `json:"results"`
	Status  int           `json:"status"`
}

type bcraSnapshot struct {
	Fecha   string      `json:"fecha"`
	Detalle []bcraEntry `json:"detalle"`
}

//nolint:tagliatelle,misspell // External API field names (Spanish "descripcion").
type bcraEntry struct {
	CodigoMoneda   string  `json:"codigoMoneda"`
	Descripcion    string  `json:"descripcion"`
	TipoPase       float64 `json:"tipoPase"`
	TipoCotizacion float64 `json:"tipoCotizacion"`
}

type BCRAProvider struct {
	client *http.Client
	url    string
}

func NewBCRAProvider(timeout time.Duration) *BCRAProvider {
	return &BCRAProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: bcraURL,
	}
}

func (p *BCRAProvider) Name() string {
	return "BCRA (ARS)"
}

func (p *BCRAProvider) Interval() time.Duration {
	return time.Hour * 3
}

func (p *BCRAProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
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

	var apiResp bcraResponse

	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	if apiResp.Status != 0 && apiResp.Status != bcraStatusOK {
		return nil, fmt.Errorf("BCRA returned application status %d", apiResp.Status)
	}

	if apiResp.Results == nil {
		return nil, fmt.Errorf("BCRA response missing results")
	}

	if apiResp.Results.Fecha == "" {
		return nil, fmt.Errorf("BCRA snapshot missing fecha")
	}

	if len(apiResp.Results.Detalle) == 0 {
		return nil, fmt.Errorf("BCRA snapshot detalle is empty")
	}

	var (
		entry *bcraEntry
		found bool
	)

	for i := range apiResp.Results.Detalle {
		if apiResp.Results.Detalle[i].CodigoMoneda == bcraUSDCode {
			entry = &apiResp.Results.Detalle[i]
			found = true

			break
		}
	}

	if !found {
		return nil, fmt.Errorf("BCRA snapshot missing USD entry")
	}

	if entry.TipoCotizacion <= 0 {
		return nil, fmt.Errorf("BCRA USD rate is non-positive: %v", entry.TipoCotizacion)
	}

	loc, err := time.LoadLocation(bcraTimezoneName)
	if err != nil {
		return nil, fmt.Errorf("unable to load Buenos Aires timezone: %w", err)
	}

	asOf, err := time.ParseInLocation(bcraDateLayout, apiResp.Results.Fecha, loc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse fecha %q: %w", apiResp.Results.Fecha, err)
	}

	return []*types.ExchangeRate{
		{
			AsOf:      asOf.UTC(),
			FetchedAt: time.Now().UTC(),
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyARS,
			RateType:  types.RateTypeMID,
			Source:    types.SourceBCRA,
			Rate:      math.Round(entry.TipoCotizacion*1e4) / 1e4,
		},
	}, nil
}
