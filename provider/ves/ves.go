package ves

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/sig-0/fxrates/provider/currencies"
	"github.com/sig-0/fxrates/storage/types"
)

var errInvalidRate = errors.New("invalid rate")

var BCVSource types.Source = "BCV"

// BCVProvider is the BCV website scraping provider
type BCVProvider struct {
	client *http.Client
	url    string
}

// NewBCVProvider creates a new instance of the BCV website provider
func NewBCVProvider(url string, timeout time.Duration) *BCVProvider {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // Fine to ignore
	}

	return &BCVProvider{
		client: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
		url: url,
	}
}

func (p *BCVProvider) Name() string {
	return "BCV"
}

func (p *BCVProvider) Interval() time.Duration {
	return time.Hour * 24 // the rate is updated daily
}

func (p *BCVProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	// Prepare the request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("unable to create new GET request: %w", err)
	}

	// Execute the request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to execute GET request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code received: %d", resp.StatusCode)
	}

	// Construct document for parsing
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to construct query doc: %w", err)
	}

	fetchCurrencyRate := func(currencyID string) (float64, error) {
		sel := doc.Find("#" + currencyID)

		if sel.Length() == 0 {
			return 0, fmt.Errorf("missing element #%s", currencyID)
		}

		txt := sel.Find(".col-sm-6.col-xs-6.centrado").First().Text()
		if strings.TrimSpace(txt) == "" {
			txt = sel.Find(".centrado").First().Text()
		}

		txt = strings.TrimSpace(txt)

		v, err := parseBCVNumber(txt)
		if err != nil {
			return 0, fmt.Errorf("unable to parse rate value for %s: %w", currencyID, err)
		}

		return math.Round(v*1e4) / 1e4, nil
	}

	var (
		fetchTime = time.Now().UTC()
		ids       = []string{
			"dolar",
			"euro",
			"yuan",
			"lira",
			"rublo",
		}

		exchangeRates = make([]*types.ExchangeRate, 0, len(ids))

		effectiveDate = fetchTime
	)

	// Fetch as-of date
	parsedEffectiveDate := parseEffectiveDate(doc)
	if parsedEffectiveDate != nil {
		effectiveDate = *parsedEffectiveDate
	}

	for _, id := range ids {
		rate, err := fetchCurrencyRate(id)
		if err != nil {
			// TODO log?
			continue
		}

		exchangeRate := &types.ExchangeRate{
			AsOf:      effectiveDate,
			FetchedAt: fetchTime,
			Base:      idToCurrency(id),
			Target:    currencies.VES,
			RateType:  types.RateTypeMID,
			Source:    BCVSource,
			Rate:      rate,
		}

		exchangeRates = append(exchangeRates, exchangeRate)
	}

	return exchangeRates, nil
}

// parseBCVNumber parses the rate number from the BCV website
func parseBCVNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errInvalidRate
	}

	// BCV typically uses comma as decimal separator and no thousands:
	// "1.234,56" -> "1234.56"
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("unable to parse rate %q: %w", s, err)
	}

	return f, nil
}

// parseEffectiveDate parses the "Fecha Valor" date on the BCV website
func parseEffectiveDate(doc *goquery.Document) *time.Time {
	// Best source: the machine-readable datetime
	sel := doc.Find(`span.date-display-single[property="dc:date"]`).First()
	if sel.Length() == 0 {
		sel = doc.Find("span.date-display-single").First()
	}

	if sel.Length() == 0 {
		return nil
	}

	if content, ok := sel.Attr("content"); ok && strings.TrimSpace(content) != "" {
		// Example: "2026-01-13T00:00:00-04:00"
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(content)); err == nil {
			u := t.UTC()

			return &u
		}
	}

	// Fallback: parse the rendered Spanish text
	txt := strings.TrimSpace(sel.Text())
	if txt == "" {
		return nil
	}

	t, err := parseBCVDate(txt)
	if err != nil {
		return nil
	}

	u := t.UTC()

	return &u
}

// parseBCVDate parses the date on the BCV website (effective date)
func parseBCVDate(s string) (time.Time, error) {
	// Example: "Martes, 13 Enero 2026"
	// We ignore day-of-week if present.
	s = strings.TrimSpace(s)
	if i := strings.Index(s, ","); i != -1 {
		s = strings.TrimSpace(s[i+1:])
	}

	parts := strings.Fields(s)
	if len(parts) < 3 {
		return time.Time{}, fmt.Errorf("date format is invalid %q", s)
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}

	month := strings.ToLower(parts[1])

	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse effective date year: %w", err)
	}

	months := map[string]time.Month{
		"enero":      time.January,
		"febrero":    time.February,
		"marzo":      time.March,
		"abril":      time.April,
		"mayo":       time.May,
		"junio":      time.June,
		"julio":      time.July,
		"agosto":     time.August,
		"septiembre": time.September,
		"setiembre":  time.September,
		"octubre":    time.October,
		"noviembre":  time.November,
		"diciembre":  time.December,
	}

	mo, ok := months[month]
	if !ok {
		return time.Time{}, fmt.Errorf("month is invalid %q", month)
	}

	return time.Date(year, mo, day, 0, 0, 0, 0, time.UTC), nil
}

// idToCurrency maps the hardcoded BCV website currency section ID
// to a common currency type
func idToCurrency(id string) types.Currency {
	switch id {
	case "dolar":
		return currencies.USD
	case "euro":
		return currencies.EUR
	case "yuan":
		return currencies.CNY
	case "lira":
		return currencies.TRY
	case "rublo":
		return currencies.RUB
	default:
		return "XXX"
	}
}
