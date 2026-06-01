package crypto

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

	"github.com/sig-0/fxrates/storage/types"
)

const binanceP2PURL = "https://p2p.binance.com/bapi/c2c/v2/friendly/c2c/adv/search"

type BinanceP2PConfig struct {
	Asset    types.Currency
	Fiat     types.Currency
	Source   types.Source
	Name     string
	Interval time.Duration
	Timeout  time.Duration
	Filter   BinanceP2PFilterOpts
}

type BinanceP2PFilterOpts struct {
	MinOrders        int
	MinFinishRate    float64
	MinAvailable     float64
	TypicalAmount    float64
	RelaxedMinOrders int
	RelaxedMinFinish float64
	TopN             int
}

func DefaultBinanceP2PFilter() BinanceP2PFilterOpts {
	return BinanceP2PFilterOpts{
		MinOrders:        50,
		MinFinishRate:    0.95,
		MinAvailable:     50,
		TypicalAmount:    100,
		RelaxedMinOrders: 20,
		RelaxedMinFinish: 0.90,
		TopN:             12,
	}
}

//nolint:tagliatelle // Binance API uses camelCase.
type binanceP2PRequest struct {
	Asset     types.Currency `json:"asset"`
	Fiat      types.Currency `json:"fiat"`
	TradeType types.RateType `json:"tradeType"`
	Rows      int            `json:"rows"`
	Page      int            `json:"page"`
}

type binanceP2PResponse struct {
	Data []binanceP2POffer `json:"data"`
}

type binanceP2POffer struct {
	Adv        binanceP2PAdv        `json:"adv"`
	Advertiser binanceP2PAdvertiser `json:"advertiser"`
}

//nolint:tagliatelle // Binance API uses camelCase.
type binanceP2PAdv struct {
	Price                string `json:"price"`
	MinSingleTransAmount string `json:"minSingleTransAmount"`
	MaxSingleTransAmount string `json:"maxSingleTransAmount"`
	SurplusAmount        string `json:"surplusAmount"`
	TradableQuantity     string `json:"tradableQuantity"`
}

//nolint:tagliatelle // Binance API uses camelCase.
type binanceP2PAdvertiser struct {
	MonthOrderCount int     `json:"monthOrderCount"`
	MonthFinishRate float64 `json:"monthFinishRate"`
}

type BinanceP2PProvider struct {
	client *http.Client
	url    string
	cfg    BinanceP2PConfig
	filter BinanceP2PFilterOpts
}

func NewBinanceP2P(cfg BinanceP2PConfig) *BinanceP2PProvider {
	filter := cfg.Filter
	if filter == (BinanceP2PFilterOpts{}) {
		filter = DefaultBinanceP2PFilter()
	}

	return &BinanceP2PProvider{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		cfg:    cfg,
		url:    binanceP2PURL,
		filter: filter,
	}
}

func (p *BinanceP2PProvider) Name() string {
	return p.cfg.Name
}

func (p *BinanceP2PProvider) Interval() time.Duration {
	return p.cfg.Interval
}

func (p *BinanceP2PProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	fetchTime := time.Now().UTC()

	buyPrice, err := p.fetchMedianPrice(ctx, types.RateTypeBUY)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch BUY price: %w", err)
	}

	sellPrice, err := p.fetchMedianPrice(ctx, types.RateTypeSELL)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch SELL price: %w", err)
	}

	return []*types.ExchangeRate{
		{
			AsOf:      fetchTime,
			FetchedAt: fetchTime,
			Base:      p.cfg.Asset,
			Target:    p.cfg.Fiat,
			RateType:  types.RateTypeBUY,
			Source:    p.cfg.Source,
			Rate:      buyPrice,
		},
		{
			AsOf:      fetchTime,
			FetchedAt: fetchTime,
			Base:      p.cfg.Asset,
			Target:    p.cfg.Fiat,
			RateType:  types.RateTypeSELL,
			Source:    p.cfg.Source,
			Rate:      sellPrice,
		},
	}, nil
}

func (p *BinanceP2PProvider) fetchMedianPrice(
	ctx context.Context,
	tradeType types.RateType,
) (float64, error) {
	offers, err := p.fetchOffers(ctx, tradeType)
	if err != nil {
		return 0, err
	}

	filtered := filterBinanceOffers(
		offers,
		p.filter.MinOrders,
		p.filter.MinFinishRate,
		p.filter.MinAvailable,
		p.filter.TypicalAmount,
	)

	if len(filtered) < p.filter.TopN {
		if relaxed := filterBinanceOffers(
			offers,
			p.filter.RelaxedMinOrders,
			p.filter.RelaxedMinFinish,
			p.filter.MinAvailable,
			p.filter.TypicalAmount,
		); len(relaxed) > len(filtered) {
			filtered = relaxed
		}
	}

	if len(filtered) == 0 {
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

	if p.filter.TopN > 0 && len(filtered) > p.filter.TopN {
		filtered = filtered[:p.filter.TopN]
	}

	prices := make([]float64, len(filtered))
	for i, offer := range filtered {
		prices[i] = offer.price
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("no valid prices found for %s", tradeType)
	}

	return math.Round(binanceMedian(prices)*1e4) / 1e4, nil
}

func (p *BinanceP2PProvider) fetchOffers(
	ctx context.Context,
	tradeType types.RateType,
) ([]binanceOffer, error) {
	offers := make([]binanceOffer, 0, 30)

	for page := 1; page <= 3; page++ {
		reqBody := binanceP2PRequest{
			Asset:     p.cfg.Asset,
			Fiat:      p.cfg.Fiat,
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
			price, ok := binanceParseFloat(offer.Adv.Price)
			if !ok {
				continue
			}

			var (
				minLimit, _ = binanceParseFloat(offer.Adv.MinSingleTransAmount)
				maxLimit, _ = binanceParseFloat(offer.Adv.MaxSingleTransAmount)
			)

			available, ok := binanceParseFloat(offer.Adv.SurplusAmount)
			if !ok {
				available, _ = binanceParseFloat(offer.Adv.TradableQuantity)
			}

			var (
				finishRate = binanceNormalizeFinishRate(offer.Advertiser.MonthFinishRate)
				orders     = offer.Advertiser.MonthOrderCount
			)

			offers = append(offers, binanceOffer{
				price:      price,
				minLimit:   minLimit,
				maxLimit:   maxLimit,
				available:  available,
				orders:     orders,
				finishRate: finishRate,
				quality:    binanceWilsonLowerBound(finishRate, orders),
			})
		}
	}

	if len(offers) == 0 {
		return nil, fmt.Errorf("no valid offers found for %s", tradeType)
	}

	return offers, nil
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

func filterBinanceOffers(
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

func binanceNormalizeFinishRate(rate float64) float64 {
	if rate <= 0 {
		return 0
	}

	if rate > 1 {
		return rate / 100
	}

	return rate
}

func binanceWilsonLowerBound(rate float64, n int) float64 {
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

func binanceParseFloat(value string) (float64, bool) {
	if value == "" {
		return 0, false
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}

	return parsed, true
}

func binanceMedian(values []float64) float64 {
	sort.Float64s(values)

	n := len(values)
	if n%2 == 0 {
		return (values[n/2-1] + values[n/2]) / 2
	}

	return values[n/2]
}
