package eur

import (
	"context"
	"encoding/xml"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

const ecbURL = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"

// ecbEnvelope is the root XML element.
type ecbEnvelope struct {
	XMLName xml.Name    `xml:"Envelope"`
	Cube    ecbTimeCube `xml:"Cube>Cube"`
}

// ecbTimeCube is the date-bearing Cube element.
type ecbTimeCube struct {
	Time  string         `xml:"time,attr"`
	Rates []ecbRateEntry `xml:"Cube"`
}

// ecbRateEntry is a single currency rate element.
type ecbRateEntry struct {
	Currency string `xml:"currency,attr"`
	Rate     string `xml:"rate,attr"`
}

// ECBProvider fetches USD/EUR reference rate from the ECB daily XML feed.
type ECBProvider struct {
	client *http.Client
	url    string
}

// NewECBProvider creates a new instance of the ECB provider.
func NewECBProvider(timeout time.Duration) *ECBProvider {
	return &ECBProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: ecbURL,
	}
}

func (p *ECBProvider) Name() string {
	return "ECB (EUR)"
}

func (p *ECBProvider) Interval() time.Duration {
	return time.Hour * 3
}

func (p *ECBProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
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

	var envelope ecbEnvelope

	if err = xml.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("unable to decode XML: %w", err)
	}

	asOf, err := time.Parse("2006-01-02", envelope.Cube.Time)
	if err != nil {
		return nil, fmt.Errorf("unable to parse date %q: %w", envelope.Cube.Time, err)
	}

	for _, entry := range envelope.Cube.Rates {
		if entry.Currency != "USD" {
			continue
		}

		eurPerUSD, err := strconv.ParseFloat(entry.Rate, 64)
		if err != nil {
			return nil, fmt.Errorf("unable to parse rate %q: %w", entry.Rate, err)
		}

		if eurPerUSD == 0 {
			return nil, fmt.Errorf("USD rate is zero")
		}

		// ECB publishes 1 EUR = X USD, so invert to get USD/EUR.
		usdToEUR := 1 / eurPerUSD

		return []*types.ExchangeRate{
			{
				AsOf:      asOf.UTC(),
				FetchedAt: time.Now().UTC(),
				Base:      types.CurrencyUSD,
				Target:    types.CurrencyEUR,
				RateType:  types.RateTypeMID,
				Source:    types.SourceECB,
				Rate:      math.Round(usdToEUR*1e4) / 1e4,
			},
		}, nil
	}

	return nil, fmt.Errorf("USD rate not found in ECB response")
}
