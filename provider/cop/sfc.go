package cop

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/sig-0/fxrates/provider/currencies"
	"github.com/sig-0/fxrates/storage/types"
)

var SFCSource types.Source = "SFC"

const sfcURL = "https://www.datos.gov.co/resource/32sa-8pi3.json?$order=vigenciadesde%20DESC&$limit=1"

//nolint:tagliatelle // External API field names.
type sfcRecord struct {
	Valor         string `json:"valor"`
	VigenciaDesde string `json:"vigenciadesde"`
}

// SFCProvider fetches USD/COP TRM from datos.gov.co (Socrata API).
type SFCProvider struct {
	client *http.Client
	url    string
}

// NewSFCProvider creates a new instance of the SFC TRM provider.
func NewSFCProvider(timeout time.Duration) *SFCProvider {
	return &SFCProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: sfcURL,
	}
}

func (p *SFCProvider) Name() string {
	return "SFC (COP)"
}

func (p *SFCProvider) Interval() time.Duration {
	return time.Hour * 3
}

func (p *SFCProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
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

	var records []sfcRecord

	if err = json.NewDecoder(resp.Body).Decode(&records); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no TRM records returned")
	}

	record := records[0]

	rate, err := strconv.ParseFloat(record.Valor, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse rate %q: %w", record.Valor, err)
	}

	asOf, err := time.Parse("2006-01-02T15:04:05.000", record.VigenciaDesde)
	if err != nil {
		return nil, fmt.Errorf("unable to parse date %q: %w", record.VigenciaDesde, err)
	}

	return []*types.ExchangeRate{
		{
			AsOf:      asOf.UTC(),
			FetchedAt: time.Now().UTC(),
			Base:      currencies.USD,
			Target:    currencies.COP,
			RateType:  types.RateTypeMID,
			Source:    SFCSource,
			Rate:      math.Round(rate*1e4) / 1e4,
		},
	}, nil
}
