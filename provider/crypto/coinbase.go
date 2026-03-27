package crypto

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

var CoinbaseSource types.Source = "Coinbase"

const coinbaseUSDCURL = "https://api.coinbase.com/v2/prices/USDC-USD/spot"

// coinbaseResponse is the top-level JSON response from the Coinbase API.
type coinbaseResponse struct {
	Data coinbasePrice `json:"data"`
}

// coinbasePrice represents the price data.
type coinbasePrice struct {
	Amount   string `json:"amount"`
	Base     string `json:"base"`
	Currency string `json:"currency"`
}

// CoinbaseUSDCProvider fetches USDC/USD spot price from Coinbase.
type CoinbaseUSDCProvider struct {
	client *http.Client
	url    string
}

// NewCoinbaseUSDCProvider creates a new instance of the Coinbase USDC provider.
func NewCoinbaseUSDCProvider(timeout time.Duration) *CoinbaseUSDCProvider {
	return &CoinbaseUSDCProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: coinbaseUSDCURL,
	}
}

func (p *CoinbaseUSDCProvider) Name() string {
	return "Coinbase (USDC)"
}

func (p *CoinbaseUSDCProvider) Interval() time.Duration {
	return time.Minute * 10
}

func (p *CoinbaseUSDCProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
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

	var apiResp coinbaseResponse

	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	rate, err := strconv.ParseFloat(apiResp.Data.Amount, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse rate %q: %w", apiResp.Data.Amount, err)
	}

	fetchTime := time.Now().UTC()

	return []*types.ExchangeRate{
		{
			AsOf:      fetchTime,
			FetchedAt: fetchTime,
			Base:      currencies.USDC,
			Target:    currencies.USD,
			RateType:  types.RateTypeMID,
			Source:    CoinbaseSource,
			Rate:      math.Round(rate*1e4) / 1e4,
		},
	}, nil
}
