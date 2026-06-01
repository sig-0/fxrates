package pen

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

const (
	bcrpBaseURL = "https://estadisticas.bcrp.gob.pe/estadisticas/series/api"
	bcrpSep     = "Sep"
)

// bcrpSpanishMonths maps the Spanish month abbreviations used by the BCRP API
// to their English equivalents so they can be parsed by time.Parse. BCRP uses
// "Set" for September, which is the common Peruvian abbreviation.
var bcrpSpanishMonths = map[string]string{
	"Ene":   "Jan",
	"Feb":   "Feb",
	"Mar":   "Mar",
	"Abr":   "Apr",
	"May":   "May",
	"Jun":   "Jun",
	"Jul":   "Jul",
	"Ago":   "Aug",
	"Set":   bcrpSep,
	bcrpSep: bcrpSep,
	"Oct":   "Oct",
	"Nov":   "Nov",
	"Dic":   "Dec",
}

// parseBCRPDate parses a date string in the BCRP "DD.Mon.YY" format, where
// Mon is a Spanish month abbreviation ("07.Abr.26")
func parseBCRPDate(s string) (time.Time, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("unexpected date format %q", s)
	}

	en, ok := bcrpSpanishMonths[parts[1]]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown Spanish month %q in date %q", parts[1], s)
	}

	parts[1] = en

	return time.Parse("02.Jan.06", strings.Join(parts, "."))
}

// bcrpResponse is the top-level JSON response from the BCRP API.
type bcrpResponse struct {
	Periods []bcrpPeriod `json:"periods"`
}

// bcrpPeriod represents a single date entry with rate values.
type bcrpPeriod struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// BCRPProvider fetches USD/PEN SBS rates from the BCRP statistics API.
type BCRPProvider struct {
	client *http.Client
}

// NewBCRPProvider creates a new instance of the BCRP provider.
func NewBCRPProvider(timeout time.Duration) *BCRPProvider {
	return &BCRPProvider{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *BCRPProvider) Name() string {
	return "BCRP (PEN)"
}

func (p *BCRPProvider) Interval() time.Duration {
	return time.Hour * 3
}

func (p *BCRPProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -7)

	url := fmt.Sprintf(
		"%s/PD04639PD-PD04640PD/json/%d-%d-%d/%d-%d-%d/",
		bcrpBaseURL,
		start.Year(), start.Month(), start.Day(),
		now.Year(), now.Month(), now.Day(),
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

	var apiResp bcrpResponse

	if err = json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("unable to decode response: %w", err)
	}

	if len(apiResp.Periods) == 0 {
		return nil, fmt.Errorf("no periods returned")
	}

	// Take the most recent period.
	period := apiResp.Periods[len(apiResp.Periods)-1]

	if len(period.Values) < 2 {
		return nil, fmt.Errorf("expected 2 values (buy/sell), got %d", len(period.Values))
	}

	// Parse the date in DD.Mon.YY format with a Spanish month
	asOf, err := parseBCRPDate(period.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to parse date %q: %w", period.Name, err)
	}

	buyRate, err := strconv.ParseFloat(period.Values[0], 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse buy rate %q: %w", period.Values[0], err)
	}

	sellRate, err := strconv.ParseFloat(period.Values[1], 64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse sell rate %q: %w", period.Values[1], err)
	}

	var (
		fetchTime = time.Now().UTC()
		midRate   = math.Round((buyRate+sellRate)/2*1e4) / 1e4
	)

	return []*types.ExchangeRate{
		{
			AsOf:      asOf.UTC(),
			FetchedAt: fetchTime,
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyPEN,
			RateType:  types.RateTypeBUY,
			Source:    types.SourceBCRP,
			Rate:      math.Round(buyRate*1e4) / 1e4,
		},
		{
			AsOf:      asOf.UTC(),
			FetchedAt: fetchTime,
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyPEN,
			RateType:  types.RateTypeSELL,
			Source:    types.SourceBCRP,
			Rate:      math.Round(sellRate*1e4) / 1e4,
		},
		{
			AsOf:      asOf.UTC(),
			FetchedAt: fetchTime,
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyPEN,
			RateType:  types.RateTypeMID,
			Source:    types.SourceBCRP,
			Rate:      midRate,
		},
	}, nil
}
