package ves

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/sig-0/fxrates/provider/currencies"
	"github.com/sig-0/fxrates/storage/types"
)

// BCVBanksProvider is the BCV website banks scraping provider
type BCVBanksProvider struct {
	client *http.Client
	url    string
}

// NewBCVBanksProvider creates a new instance of the BCV website banks provider
func NewBCVBanksProvider(url string, timeout time.Duration) *BCVBanksProvider {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // Fine to ignore
	}

	return &BCVBanksProvider{
		client: &http.Client{
			Timeout:   timeout,
			Transport: tr,
		},
		url: url,
	}
}

func (p *BCVBanksProvider) Name() string {
	return "BCV Banks"
}

func (p *BCVBanksProvider) Interval() time.Duration {
	return time.Hour * 24 // the rates are updated daily
}

func (p *BCVBanksProvider) Fetch(ctx context.Context) ([]*types.ExchangeRate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to parse html: %w", err)
	}

	type row struct {
		asOfUTC time.Time // UTC midnight of the effective date
		bank    string
		buy     float64
		sell    float64
	}

	var (
		loc        = caracasLocation()
		nowCaracas = time.Now().In(loc)
		todayUTC   = utcMidnightOfDate(nowCaracas)
	)

	rows := make([]row, 0, 128)

	doc.Find("table.views-table tbody tr").Each(func(_ int, tr *goquery.Selection) {
		dateSel := tr.Find("td.views-field-field-fecha-del-indicador span.date-display-single").First()

		content, ok := dateSel.Attr("content")
		if !ok || strings.TrimSpace(content) == "" {
			return
		}

		t, err := time.Parse(time.RFC3339, strings.TrimSpace(content))
		if err != nil {
			return
		}

		asOfUTC := utcMidnightOfDate(t.In(loc))
		if asOfUTC.After(todayUTC) {
			return
		}

		bank := strings.TrimSpace(tr.Find("td.views-field-views-conditional").First().Text())
		if bank == "" {
			return
		}

		var (
			buyTxt  = strings.TrimSpace(tr.Find("td.views-field-field-tasa-compra").First().Text())
			sellTxt = strings.TrimSpace(tr.Find("td.views-field-field-tasa-venta").First().Text())
		)

		if buyTxt == "" || sellTxt == "" {
			return
		}

		buy, err := parseBCVNumber(buyTxt)
		if err != nil {
			return
		}

		sell, err := parseBCVNumber(sellTxt)
		if err != nil {
			return
		}

		rows = append(rows, row{
			asOfUTC: asOfUTC,
			bank:    bank,
			buy:     buy,
			sell:    sell,
		})
	})

	if len(rows) == 0 {
		return nil, fmt.Errorf("no table rows found")
	}

	// Pick the latest as-of date <= today (Caracas time)
	var dates []time.Time

	seen := map[int64]time.Time{}

	for _, r := range rows {
		k := r.asOfUTC.Unix()

		if _, ok := seen[k]; !ok {
			seen[k] = r.asOfUTC
			dates = append(dates, r.asOfUTC)
		}
	}

	sort.Slice(dates, func(i, j int) bool {
		return dates[i].After(dates[j])
	})

	chosen := dates[0]

	// Sanity check, don't accept something ancient
	if todayUTC.Sub(chosen) > 7*24*time.Hour {
		return nil, fmt.Errorf("latest available date too old: %s", chosen.Format("2006-01-02"))
	}

	var (
		fetchTime = time.Now().UTC()
		out       = make([]*types.ExchangeRate, 0, len(rows)*2)
	)

	for _, r := range rows {
		if !sameUTCDate(r.asOfUTC, chosen) {
			continue
		}

		src := types.Source(r.bank) // TODO standardize?

		out = append(
			out,
			&types.ExchangeRate{
				AsOf:      r.asOfUTC,
				FetchedAt: fetchTime,
				Base:      currencies.USD,
				Target:    currencies.VES,
				RateType:  types.RateTypeBUY,
				Source:    src,
				Rate:      r.buy,
			},
			&types.ExchangeRate{
				AsOf:      r.asOfUTC,
				FetchedAt: fetchTime,
				Base:      currencies.USD,
				Target:    currencies.VES,
				RateType:  types.RateTypeSELL,
				Source:    src,
				Rate:      r.sell,
			},
		)
	}

	// If nothing matched chosen date, something is weird in parsing
	if len(out) == 0 {
		return nil, fmt.Errorf("no rows matched chosen date %s", chosen.Format("2006-01-02"))
	}

	return out, nil
}

func caracasLocation() *time.Location {
	loc, err := time.LoadLocation("America/Caracas")
	if err == nil {
		return loc
	}

	return time.FixedZone("VET", -4*60*60)
}

func utcMidnightOfDate(t time.Time) time.Time {
	y, m, d := t.Date()

	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sameUTCDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()

	return ay == by && am == bm && ad == bd
}
