package brl

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

var BCBSource types.Source = "BCB"

const bcbBaseURL = "https://olinda.bcb.gov.br/olinda/servico/PTAX/versao/v1/odata"

// bcbResponse is the OData response from the BCB PTAX API.
type bcbResponse struct {
	Value []bcbRate `json:"value"`
}

//nolint:tagliatelle // External API field names.
type bcbRate struct {
	DataHoraCotacao string  `json:"dataHoraCotacao"`
	CotacaoCompra   float64 `json:"cotacaoCompra"`
	CotacaoVenda    float64 `json:"cotacaoVenda"`
}

// BCBProvider fetches USD/BRL PTAX rates from the Banco Central do Brasil OData API.
type BCBProvider struct {
	client *http.Client
}

// NewBCBProvider creates a new instance of the BCB PTAX provider.
func NewBCBProvider(timeout time.Duration) *BCBProvider {
	return &BCBProvider{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *BCBProvider) Name() string {
	return "BCB (BRL)"
}

func (p *BCBProvider) Interval() time.Duration {
	return time.Hour * 3
}

func (p *BCBProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	brasilia, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return nil, fmt.Errorf("unable to load Brasilia timezone: %w", err)
	}

	today := time.Now().In(brasilia)

	// Try today first, then look back up to 4 days to cover weekends and holidays.
	for i := range 5 {
		date := today.AddDate(0, 0, -i)

		rates, err := p.fetchForDate(ctx, date, brasilia)
		if err != nil {
			return nil, err
		}

		if len(rates) > 0 {
			return rates, nil
		}
	}

	return nil, fmt.Errorf("no PTAX rates available for the last 5 days")
}

func (p *BCBProvider) fetchForDate(
	ctx context.Context,
	date time.Time,
	loc *time.Location,
) ([]*types.ExchangeRate, error) {
	url := fmt.Sprintf(
		"%s/CotacaoDolarDia(dataCotacao=@dataCotacao)?@dataCotacao='%s'&$format=json",
		bcbBaseURL,
		date.Format("01-02-2006"),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
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

	var apiResp bcbResponse

	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	if len(apiResp.Value) == 0 {
		return nil, nil
	}

	entry := apiResp.Value[len(apiResp.Value)-1]

	asOf, err := time.ParseInLocation("2006-01-02 15:04:05.999", entry.DataHoraCotacao, loc)
	if err != nil {
		return nil, fmt.Errorf("unable to parse date %q: %w", entry.DataHoraCotacao, err)
	}

	var (
		fetchTime = time.Now().UTC()
		midRate   = math.Round((entry.CotacaoCompra+entry.CotacaoVenda)/2*1e4) / 1e4
	)

	return []*types.ExchangeRate{
		{
			AsOf:      asOf.UTC(),
			FetchedAt: fetchTime,
			Base:      currencies.USD,
			Target:    currencies.BRL,
			RateType:  types.RateTypeBUY,
			Source:    BCBSource,
			Rate:      math.Round(entry.CotacaoCompra*1e4) / 1e4,
		},
		{
			AsOf:      asOf.UTC(),
			FetchedAt: fetchTime,
			Base:      currencies.USD,
			Target:    currencies.BRL,
			RateType:  types.RateTypeSELL,
			Source:    BCBSource,
			Rate:      math.Round(entry.CotacaoVenda*1e4) / 1e4,
		},
		{
			AsOf:      asOf.UTC(),
			FetchedAt: fetchTime,
			Base:      currencies.USD,
			Target:    currencies.BRL,
			RateType:  types.RateTypeMID,
			Source:    BCBSource,
			Rate:      midRate,
		},
	}, nil
}
