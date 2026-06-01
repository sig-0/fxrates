package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sig-0/fxrates/storage/types"
)

const testNameARS = "Binance P2P (USDT/ARS)"

func TestBinanceFilterOffers(t *testing.T) {
	t.Parallel()

	offers := []binanceOffer{
		{price: 100, minLimit: 10, maxLimit: 1000, available: 100, orders: 100, finishRate: 0.99},
		{price: 101, minLimit: 10, maxLimit: 1000, available: 100, orders: 10, finishRate: 0.99},
		{price: 102, minLimit: 10, maxLimit: 1000, available: 100, orders: 100, finishRate: 0.5},
		{price: 103, minLimit: 10, maxLimit: 1000, available: 1, orders: 100, finishRate: 0.99},
		{price: 104, minLimit: 200, maxLimit: 1000, available: 100, orders: 100, finishRate: 0.99},
		{price: 105, minLimit: 10, maxLimit: 50, available: 100, orders: 100, finishRate: 0.99},
	}

	t.Run("strict thresholds", func(t *testing.T) {
		t.Parallel()

		filtered := filterBinanceOffers(offers, 50, 0.95, 50, 100)

		require.Len(t, filtered, 1)
		assert.InEpsilon(t, 100.0, filtered[0].price, 1e-9)
	})

	t.Run("relaxed thresholds keep more", func(t *testing.T) {
		t.Parallel()

		filtered := filterBinanceOffers(offers, 20, 0.90, 50, 100)
		assert.GreaterOrEqual(t, len(filtered), 1)
	})

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, filterBinanceOffers(nil, 1, 0.5, 0, 0))
	})
}

func TestBinanceWilsonLowerBound(t *testing.T) {
	t.Parallel()

	assert.InDelta(t, 0.0, binanceWilsonLowerBound(0.97, 0), 1e-9)
	assert.InDelta(t, 0.0, binanceWilsonLowerBound(0.97, -5), 1e-9)

	score := binanceWilsonLowerBound(1.0, 100)
	assert.Greater(t, score, 0.9)
	assert.Less(t, score, 1.0)

	low := binanceWilsonLowerBound(0.5, 10)
	high := binanceWilsonLowerBound(0.5, 1000)
	assert.Less(t, low, high)
}

func TestBinanceMedian(t *testing.T) {
	t.Parallel()

	assert.InEpsilon(t, 2.0, binanceMedian([]float64{1, 2, 3}), 1e-9)
	assert.InEpsilon(t, 2.5, binanceMedian([]float64{1, 2, 3, 4}), 1e-9)
	assert.InEpsilon(t, 5.0, binanceMedian([]float64{5}), 1e-9)
}

func TestBinanceNormalizeFinishRate(t *testing.T) {
	t.Parallel()

	assert.InDelta(t, 0.0, binanceNormalizeFinishRate(0), 1e-9)
	assert.InDelta(t, 0.0, binanceNormalizeFinishRate(-0.5), 1e-9)
	assert.InEpsilon(t, 0.97, binanceNormalizeFinishRate(0.97), 1e-9)
	assert.InEpsilon(t, 0.97, binanceNormalizeFinishRate(97), 1e-9)
}

func TestBinanceParseFloat(t *testing.T) {
	t.Parallel()

	v, ok := binanceParseFloat("1.25")
	assert.True(t, ok)
	assert.InEpsilon(t, 1.25, v, 1e-9)

	_, ok = binanceParseFloat("")
	assert.False(t, ok)

	_, ok = binanceParseFloat("abc")
	assert.False(t, ok)
}

func buildBinanceOffer(price float64) binanceP2POffer {
	return binanceP2POffer{
		Adv: binanceP2PAdv{
			Price:                fmt.Sprintf("%.4f", price),
			MinSingleTransAmount: "10",
			MaxSingleTransAmount: "10000",
			SurplusAmount:        "1000",
			TradableQuantity:     "1000",
		},
		Advertiser: binanceP2PAdvertiser{
			MonthOrderCount: 200,
			MonthFinishRate: 0.99,
		},
	}
}

// pagedBinanceServer returns three pages per tradeType. Page 1 returns 10 offers,
// page 2 returns 10 offers with different prices, page 3 returns empty (early break path).
func pagedBinanceServer(t *testing.T) *httptest.Server {
	t.Helper()

	page1Buy := make([]binanceP2POffer, 0, 10)
	page2Buy := make([]binanceP2POffer, 0, 10)
	page1Sell := make([]binanceP2POffer, 0, 10)
	page2Sell := make([]binanceP2POffer, 0, 10)

	for i := range 10 {
		page1Buy = append(page1Buy, buildBinanceOffer(1000+float64(i)))
		page2Buy = append(page2Buy, buildBinanceOffer(1020+float64(i)))
		page1Sell = append(page1Sell, buildBinanceOffer(1100+float64(i)))
		page2Sell = append(page2Sell, buildBinanceOffer(1120+float64(i)))
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req binanceP2PRequest

		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		var data []binanceP2POffer

		switch {
		case req.Page == 1 && req.TradeType == types.RateTypeBUY:
			data = page1Buy
		case req.Page == 2 && req.TradeType == types.RateTypeBUY:
			data = page2Buy
		case req.Page == 1 && req.TradeType == types.RateTypeSELL:
			data = page1Sell
		case req.Page == 2 && req.TradeType == types.RateTypeSELL:
			data = page2Sell
		default:
			data = nil
		}

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(binanceP2PResponse{Data: data}))
	}))
}

func TestBinanceP2PFetch_VES(t *testing.T) {
	t.Parallel()

	srv := pagedBinanceServer(t)
	t.Cleanup(srv.Close)

	p := NewBinanceP2P(BinanceP2PConfig{
		Asset:    types.CurrencyUSDT,
		Fiat:     types.CurrencyVES,
		Source:   types.SourceBinanceP2P,
		Name:     "Binance P2P (USDT/VES)",
		Interval: time.Minute * 10,
		Timeout:  time.Second * 5,
	})
	p.url = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	rates, err := p.Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, rates, 2)

	for _, r := range rates {
		assert.Equal(t, types.CurrencyUSDT, r.Base)
		assert.Equal(t, types.CurrencyVES, r.Target)
		assert.Equal(t, types.SourceBinanceP2P, r.Source)
		assert.False(t, r.AsOf.IsZero())
		assert.False(t, r.FetchedAt.IsZero())
		assert.Positive(t, r.Rate)
	}

	assert.Equal(t, types.RateTypeBUY, rates[0].RateType)
	assert.Equal(t, types.RateTypeSELL, rates[1].RateType)

	// Median of page-1 alone would be 1004.5 (10 items, ascending). Picking up two
	// offers from page 2 brings the median to 1005.5 (12 items, top-N=12). Use that
	// gap to prove page 2 was actually fetched.
	assert.InEpsilon(t, 1005.5, rates[0].Rate, 1e-9, "median must reflect offers from pages 1 and 2")

	// SELL sorts descending then takes top 12, then median sorts ascending again:
	// [1108, 1109, 1120..1129] → indices 5 and 6 = (1123 + 1124)/2 = 1123.5.
	assert.InEpsilon(t, 1123.5, rates[1].Rate, 1e-9, "SELL median must reflect offers from pages 1 and 2")
}

func TestBinanceP2PFetch_ARS(t *testing.T) {
	t.Parallel()

	srv := pagedBinanceServer(t)
	t.Cleanup(srv.Close)

	p := NewBinanceP2P(BinanceP2PConfig{
		Asset:    types.CurrencyUSDT,
		Fiat:     types.CurrencyARS,
		Source:   types.SourceBinanceP2P,
		Name:     testNameARS,
		Interval: time.Minute * 10,
		Timeout:  time.Second * 5,
	})
	p.url = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	t.Cleanup(cancel)

	rates, err := p.Fetch(ctx)
	require.NoError(t, err)
	require.Len(t, rates, 2)

	for _, r := range rates {
		assert.Equal(t, types.CurrencyUSDT, r.Base)
		assert.Equal(t, types.CurrencyARS, r.Target)
		assert.Equal(t, types.SourceBinanceP2P, r.Source)
	}
}

func TestBinanceP2PFetch_NoOffers(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(binanceP2PResponse{Data: nil}))
	}))
	t.Cleanup(srv.Close)

	p := NewBinanceP2P(BinanceP2PConfig{
		Asset:    types.CurrencyUSDT,
		Fiat:     types.CurrencyARS,
		Source:   types.SourceBinanceP2P,
		Name:     testNameARS,
		Interval: time.Minute * 10,
		Timeout:  time.Second * 5,
	})
	p.url = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	_, err := p.Fetch(ctx)
	assert.Error(t, err)
}

func TestBinanceP2PFetch_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	p := NewBinanceP2P(BinanceP2PConfig{
		Asset:    types.CurrencyUSDT,
		Fiat:     types.CurrencyARS,
		Source:   types.SourceBinanceP2P,
		Name:     testNameARS,
		Interval: time.Minute * 10,
		Timeout:  time.Second * 5,
	})
	p.url = srv.URL

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	t.Cleanup(cancel)

	_, err := p.Fetch(ctx)
	assert.Error(t, err)
}

func TestNewBinanceP2P_DefaultsApplied(t *testing.T) {
	t.Parallel()

	p := NewBinanceP2P(BinanceP2PConfig{
		Asset:  types.CurrencyUSDT,
		Fiat:   types.CurrencyARS,
		Source: types.SourceBinanceP2P,
	})

	assert.Equal(t, DefaultBinanceP2PFilter(), p.filter)
}

func TestNewBinanceP2P_CustomFilterRespected(t *testing.T) {
	t.Parallel()

	custom := BinanceP2PFilterOpts{
		MinOrders:        10,
		MinFinishRate:    0.5,
		MinAvailable:     1,
		TypicalAmount:    1,
		RelaxedMinOrders: 5,
		RelaxedMinFinish: 0.4,
		TopN:             3,
	}

	p := NewBinanceP2P(BinanceP2PConfig{
		Asset:  types.CurrencyUSDT,
		Fiat:   types.CurrencyARS,
		Source: types.SourceBinanceP2P,
		Filter: custom,
	})

	assert.Equal(t, custom, p.filter)
}
