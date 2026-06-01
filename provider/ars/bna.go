package ars

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/sig-0/fxrates/storage/types"
)

const (
	bnaURL                  = "https://www.bna.com.ar/Personas"
	bnaBilletesSectionID    = "#billetes"
	bnaBilletesSectionTitle = "Cotización Billetes"
	bnaBilletesAnchorSel    = `a[href="#billetes"]`
	bnaDolarRowLabel        = "Dolar U.S.A"
	bnaHoraPrefix           = "Hora Actualización:"
	bnaDateLayout           = "2/1/2006"
	bnaHourLayout           = "15:04"
	bnaUserAgent            = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
		"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var errInvalidARSNumber = errors.New("invalid ARS number")

type BNAProvider struct {
	client *http.Client
	url    string
}

func NewBNAProvider(timeout time.Duration) *BNAProvider {
	return &BNAProvider{
		client: &http.Client{
			Timeout: timeout,
		},
		url: bnaURL,
	}
}

func (p *BNAProvider) Name() string {
	return "BNA (ARS)"
}

func (p *BNAProvider) Interval() time.Duration {
	return time.Hour
}

func (p *BNAProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("unable to create GET request: %w", err)
	}

	req.Header.Set("User-Agent", bnaUserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to execute GET request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code received: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to construct query doc: %w", err)
	}

	return parseBNADocument(doc, time.Now().UTC())
}

func parseBNADocument(doc *goquery.Document, fetchTime time.Time) ([]*types.ExchangeRate, error) {
	anchor := doc.Find(bnaBilletesAnchorSel).FilterFunction(func(_ int, s *goquery.Selection) bool {
		return strings.TrimSpace(s.Text()) == bnaBilletesSectionTitle
	}).First()
	if anchor.Length() == 0 {
		return nil, fmt.Errorf("BNA Billetes section header missing")
	}

	section := doc.Find(bnaBilletesSectionID).First()
	if section.Length() == 0 {
		return nil, fmt.Errorf("BNA Billetes section (%s) missing", bnaBilletesSectionID)
	}

	table := section.Find("table").First()
	if table.Length() == 0 {
		return nil, fmt.Errorf("BNA Billetes table missing")
	}

	buy, sell, ok := findUSDRow(table)
	if !ok {
		return nil, fmt.Errorf("BNA Billetes %q row missing", bnaDolarRowLabel)
	}

	buyRate, err := parseARSNumber(buy)
	if err != nil {
		return nil, fmt.Errorf("unable to parse BNA buy rate %q: %w", buy, err)
	}

	sellRate, err := parseARSNumber(sell)
	if err != nil {
		return nil, fmt.Errorf("unable to parse BNA sell rate %q: %w", sell, err)
	}

	if buyRate <= 0 || sellRate <= 0 {
		return nil, fmt.Errorf("BNA rate is non-positive: buy=%v sell=%v", buyRate, sellRate)
	}

	dateText := strings.TrimSpace(table.Find("th.fechaCot").First().Text())
	if dateText == "" {
		return nil, fmt.Errorf("BNA Billetes date node missing")
	}

	hourText, ok := findBilletesHour(section)
	if !ok {
		return nil, fmt.Errorf("BNA Billetes Hora Actualización missing")
	}

	loc, err := time.LoadLocation(bcraTimezoneName)
	if err != nil {
		return nil, fmt.Errorf("unable to load Buenos Aires timezone: %w", err)
	}

	asOf, err := parseBNATimestamp(dateText, hourText, loc)
	if err != nil {
		return nil, err
	}

	midRate := math.Round((buyRate+sellRate)/2*1e4) / 1e4

	return []*types.ExchangeRate{
		{
			AsOf:      asOf,
			FetchedAt: fetchTime,
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyARS,
			RateType:  types.RateTypeBUY,
			Source:    types.SourceBNA,
			Rate:      math.Round(buyRate*1e4) / 1e4,
		},
		{
			AsOf:      asOf,
			FetchedAt: fetchTime,
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyARS,
			RateType:  types.RateTypeSELL,
			Source:    types.SourceBNA,
			Rate:      math.Round(sellRate*1e4) / 1e4,
		},
		{
			AsOf:      asOf,
			FetchedAt: fetchTime,
			Base:      types.CurrencyUSD,
			Target:    types.CurrencyARS,
			RateType:  types.RateTypeMID,
			Source:    types.SourceBNA,
			Rate:      midRate,
		},
	}, nil
}

func findUSDRow(table *goquery.Selection) (string, string, bool) {
	var (
		buy, sell string
		found     bool
	)

	table.Find("tbody tr").EachWithBreak(func(_ int, row *goquery.Selection) bool {
		cells := row.Find("td")
		if cells.Length() < 3 {
			return true
		}

		if strings.TrimSpace(cells.Eq(0).Text()) != bnaDolarRowLabel {
			return true
		}

		buy = strings.TrimSpace(cells.Eq(1).Text())
		sell = strings.TrimSpace(cells.Eq(2).Text())
		found = true

		return false
	})

	return buy, sell, found
}

func findBilletesHour(section *goquery.Selection) (string, bool) {
	var (
		hour string
		ok   bool
	)

	section.Find(".legal").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		text := strings.TrimSpace(s.Text())
		if strings.HasPrefix(text, bnaHoraPrefix) {
			hour = strings.TrimSpace(strings.TrimPrefix(text, bnaHoraPrefix))
			ok = true

			return false
		}

		return true
	})

	return hour, ok
}

func parseBNATimestamp(dateText, hourText string, loc *time.Location) (time.Time, error) {
	date, err := time.ParseInLocation(bnaDateLayout, dateText, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse BNA date %q: %w", dateText, err)
	}

	hour, err := time.ParseInLocation(bnaHourLayout, hourText, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("unable to parse BNA hour %q: %w", hourText, err)
	}

	return time.Date(
		date.Year(), date.Month(), date.Day(),
		hour.Hour(), hour.Minute(), 0, 0,
		loc,
	).UTC(), nil
}

// parseARSNumber parses an Argentine-formatted decimal: thousands `.`, decimal `,`.
func parseARSNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errInvalidARSNumber
	}

	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q: %w", errInvalidARSNumber, s, err)
	}

	return f, nil
}
