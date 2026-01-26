//nolint:tagliatelle // Binance API uses snake case
package ves

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/sig-0/fxrates/provider/currencies"
	"github.com/sig-0/fxrates/storage/types"
)

var BinanceP2PSource types.Source = "BinanceP2P"

const binanceP2PURL = "https://p2p.binance.com/bapi/c2c/v2/friendly/c2c/adv/search"

// binanceP2PRequest is the request body for the Binance P2P API
type binanceP2PRequest struct {
	Asset     types.Currency `json:"asset"`
	Fiat      types.Currency `json:"fiat"`
	TradeType types.RateType `json:"tradeType"`
	Rows      int            `json:"rows"`
	Page      int            `json:"page"`
}

// binanceP2PResponse is the response from the Binance P2P API
type binanceP2PResponse struct {
	Data []binanceP2POffer `json:"data"`
}

type binanceP2POffer struct {
	Adv        binanceP2PAdv        `json:"adv"`
	Advertiser binanceP2PAdvertiser `json:"advertiser"`
}

type binanceP2PAdv struct {
	Price                string `json:"price"`
	MinSingleTransAmount string `json:"minSingleTransAmount"`
	MaxSingleTransAmount string `json:"maxSingleTransAmount"`
	SurplusAmount        string `json:"surplusAmount"`
	TradableQuantity     string `json:"tradableQuantity"`
}

type binanceP2PAdvertiser struct {
	MonthOrderCount int     `json:"monthOrderCount"`
	MonthFinishRate float64 `json:"monthFinishRate"`
}

type binanceOffer struct {
	price      float64
	minLimit   float64
	maxLimit   float64
	available  float64
	orders     int
	finishRate float64
	quality    float64
}

// BinanceP2PProvider fetches USDT/VES rates from Binance P2P
type BinanceP2PProvider struct {
	client *http.Client
	url    string
}

// NewBinanceP2PProvider creates a new instance of the Binance P2P provider
func NewBinanceP2PProvider(timeout time.Duration) *BinanceP2PProvider {
	return &BinanceP2PProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: binanceP2PURL,
	}
}

func (p *BinanceP2PProvider) Name() string {
	return "Binance P2P (USDT)"
}

func (p *BinanceP2PProvider) Interval() time.Duration {
	return time.Minute * 10
}

func (p *BinanceP2PProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	fetchTime := time.Now().UTC()

	// Fetch the buy price
	buyPrice, err := p.fetchMedianPrice(ctx, types.RateTypeBUY)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch BUY price: %w", err)
	}

	// Fetch the sell price
	sellPrice, err := p.fetchMedianPrice(ctx, types.RateTypeSELL)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch SELL price: %w", err)
	}

	return []*types.ExchangeRate{
		{
			AsOf:      fetchTime,
			FetchedAt: fetchTime,
			Base:      currencies.USDT,
			Target:    currencies.VES,
			RateType:  types.RateTypeBUY,
			Source:    BinanceP2PSource,
			Rate:      buyPrice,
		},
		{
			AsOf:      fetchTime,
			FetchedAt: fetchTime,
			Base:      currencies.USDT,
			Target:    currencies.VES,
			RateType:  types.RateTypeSELL,
			Source:    BinanceP2PSource,
			Rate:      sellPrice,
		},
	}, nil
}

// fetchMedianPrice fetches offers and returns the median price
func (p *BinanceP2PProvider) fetchMedianPrice(
	ctx context.Context,
	tradeType types.RateType,
) (float64, error) {
	// Fetch the seemingly best offers
	offers, err := p.fetchOffers(ctx, tradeType)
	if err != nil {
		return 0, err
	}

	// Filter out the very best for the median
	filtered := filterOffers(
		offers,
		50,
		0.95,
		50,
		100,
	)

	if len(filtered) < 12 {
		// Filter with relaxed criteria
		if relaxed := filterOffers(
			offers,
			20,
			0.90,
			50,
			100,
		); len(relaxed) > len(filtered) {
			filtered = relaxed
		}
	}

	if len(filtered) == 0 {
		// Fallback, use all offers as none match criteria
		filtered = offers
	}

	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].price != filtered[j].price {
			if tradeType == types.RateTypeBUY {
				return filtered[i].price < filtered[j].price
			}

			return filtered[i].price > filtered[j].price
		}

		return filtered[i].quality > filtered[j].quality
	})

	if len(filtered) > 12 {
		filtered = filtered[:12]
	}

	prices := make([]float64, len(filtered))
	for i, offer := range filtered {
		prices[i] = offer.price
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("no valid prices found for %s", tradeType)
	}

	return math.Round(median(prices)*1e4) / 1e4, nil
}

// fetchOffers queries Binance P2P and parses offers
func (p *BinanceP2PProvider) fetchOffers(
	ctx context.Context,
	tradeType types.RateType,
) ([]binanceOffer, error) {
	offers := make([]binanceOffer, 0, 30)

	for page := 1; page <= 3; page++ {
		reqBody := binanceP2PRequest{
			Asset:     currencies.USDT,
			Fiat:      currencies.VES,
			TradeType: tradeType,
			Rows:      10,
			Page:      page,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("unable to create POST request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("unable to execute POST request: %w", err)
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()

			return nil, fmt.Errorf("invalid status code received: %d", resp.StatusCode)
		}

		var apiResp binanceP2PResponse
		if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			resp.Body.Close()

			return nil, fmt.Errorf("unable to decode response: %w", err)
		}

		resp.Body.Close()

		if len(apiResp.Data) == 0 {
			break
		}

		for _, offer := range apiResp.Data {
			price, ok := parseFloat(offer.Adv.Price)
			if !ok {
				continue
			}

			var (
				minLimit, _ = parseFloat(offer.Adv.MinSingleTransAmount)
				maxLimit, _ = parseFloat(offer.Adv.MaxSingleTransAmount)
			)

			available, ok := parseFloat(offer.Adv.SurplusAmount)
			if !ok {
				available, _ = parseFloat(offer.Adv.TradableQuantity)
			}

			var (
				finishRate = normalizeFinishRate(offer.Advertiser.MonthFinishRate)
				orders     = offer.Advertiser.MonthOrderCount
			)

			offers = append(offers, binanceOffer{
				price:      price,
				minLimit:   minLimit,
				maxLimit:   maxLimit,
				available:  available,
				orders:     orders,
				finishRate: finishRate,
				quality:    wilsonLowerBound(finishRate, orders),
			})
		}
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no valid offers found for %s", tradeType)
	}

	return offers, nil
}

// filterOffers applies quality and limit thresholds
func filterOffers(
	offers []binanceOffer,
	minOrders int,
	minFinish float64,
	minAvailable float64,
	typicalAmount float64,
) []binanceOffer {
	filtered := make([]binanceOffer, 0, len(offers))

	for _, offer := range offers {
		if offer.orders < minOrders {
			continue
		}

		if offer.finishRate < minFinish {
			continue
		}

		if minAvailable > 0 && offer.available > 0 && offer.available < minAvailable {
			continue
		}

		if typicalAmount > 0 {
			if offer.minLimit > 0 && typicalAmount < offer.minLimit {
				continue
			}

			if offer.maxLimit > 0 && typicalAmount > offer.maxLimit {
				continue
			}
		}

		filtered = append(filtered, offer)
	}

	return filtered
}

// normalizeFinishRate ensures finish rate is 0-1
func normalizeFinishRate(rate float64) float64 {
	if rate <= 0 {
		return 0
	}

	if rate > 1 {
		return rate / 100
	}

	return rate
}

// wilsonLowerBound returns a conservative completion score
func wilsonLowerBound(rate float64, n int) float64 {
	if n <= 0 {
		return 0
	}

	var (
		z           = 1.96
		denominator = 1 + z*z/float64(n)
		center      = rate + z*z/(2*float64(n))
		adjust      = z * math.Sqrt((rate*(1-rate)+z*z/(4*float64(n)))/float64(n))
	)

	return (center - adjust) / denominator
}

// parseFloat parses a float string into a value
func parseFloat(value string) (float64, bool) {
	if value == "" {
		return 0, false
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	return parsed, true
}

// median calculates the median of a slice of float64 values
func median(values []float64) float64 {
	sort.Float64s(values)

	n := len(values)
	if n%2 == 0 {
		return (values[n/2-1] + values[n/2]) / 2
	}

	return values[n/2]
}
