package sql

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	pgStorage "github.com/sig-0/fxrates/storage/sql/gen"
	"github.com/sig-0/fxrates/storage/types"
)

type Storage struct {
	queries *pgStorage.Queries
}

func NewStorage(queries *pgStorage.Queries) *Storage {
	return &Storage{
		queries: queries,
	}
}

func (s *Storage) SaveExchangeRate(
	ctx context.Context,
	rate *types.ExchangeRate,
) error {
	arg := pgStorage.SaveExchangeRateParams{
		Base:      rate.Base.String(),
		Target:    rate.Target.String(),
		Rate:      floatToNumeric(rate.Rate),
		RateType:  rate.RateType.String(),
		Source:    rate.Source.String(),
		AsOf:      timeToTimestampz(rate.AsOf),
		FetchedAt: timeToTimestampz(rate.FetchedAt),
	}

	if err := s.queries.SaveExchangeRate(ctx, arg); err != nil {
		return fmt.Errorf("unable to save exchange rate: %w", err)
	}

	return nil
}

func (s *Storage) RateAsOf(
	ctx context.Context,
	query *types.RateQuery,
	t time.Time,
) (*types.Page[*types.ExchangeRate], error) {
	arg := pgStorage.RateAsOfParams{
		Base:   query.Base.String(),
		AsOf:   timeToTimestampz(t),
		Limit:  query.Limit,
		Offset: query.Offset,

		Target:   stringArgToText(query.Target),
		Source:   stringArgToText(query.Source),
		RateType: stringArgToText(query.RateType),
	}

	rows, err := s.queries.RateAsOf(ctx, arg)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &types.Page[*types.ExchangeRate]{
				Results: nil,
				Total:   0,
			}, nil // valid case
		}

		return nil, fmt.Errorf("unable to fetch rates: %w", err)
	}

	if len(rows) == 0 {
		return &types.Page[*types.ExchangeRate]{
			Results: nil,
			Total:   0,
		}, nil // valid case
	}

	out := make([]*types.ExchangeRate, 0, len(rows))
	for i := range rows {
		pgRate := pgStorage.ExchangeRate{
			ID:        rows[i].ID,
			Base:      rows[i].Base,
			Target:    rows[i].Target,
			Rate:      rows[i].Rate,
			RateType:  rows[i].RateType,
			Source:    rows[i].Source,
			AsOf:      rows[i].AsOf,
			FetchedAt: rows[i].FetchedAt,
		}

		out = append(out, parseExchangeRate(pgRate))
	}

	return &types.Page[*types.ExchangeRate]{
		Results: out,
		Total:   rows[0].Total,
	}, nil
}

func (s *Storage) ListSources(ctx context.Context) ([]types.Source, error) {
	results, err := s.queries.ListSources(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // valid case
		}

		return nil, fmt.Errorf("unable to fetch sources: %w", err)
	}

	if len(results) == 0 {
		return nil, nil //nolint:nilnil // valid case
	}

	out := make([]types.Source, 0, len(results))

	for _, src := range results {
		out = append(out, types.Source(src))
	}

	return out, nil
}

func (s *Storage) ListCurrencies(ctx context.Context) ([]types.Currency, error) {
	results, err := s.queries.ListCurrencies(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil //nolint:nilnil // valid case
		}

		return nil, fmt.Errorf("unable to fetch currencies: %w", err)
	}

	if len(results) == 0 {
		return nil, nil //nolint:nilnil // valid case
	}

	out := make([]types.Currency, 0, len(results))

	for _, code := range results {
		out = append(out, types.Currency(code))
	}

	return out, nil
}

// parseExchangeRate parses the postgres exchange rate to the common Go type
func parseExchangeRate(pgRate pgStorage.ExchangeRate) *types.ExchangeRate {
	if !pgRate.Rate.Valid || pgRate.Rate.Int == nil {
		return nil
	}

	return &types.ExchangeRate{
		Base:      types.Currency(pgRate.Base),
		Target:    types.Currency(pgRate.Target),
		Rate:      numericToFloat(pgRate.Rate),
		RateType:  types.RateType(pgRate.RateType),
		Source:    types.Source(pgRate.Source),
		AsOf:      timestampzToTime(pgRate.AsOf),
		FetchedAt: timestampzToTime(pgRate.FetchedAt),
	}
}

// floatToNumeric converts the float value to postgres numeric
func floatToNumeric(value float64) pgtype.Numeric {
	// round to 4dp and store as integer with exponent -4
	i := int64(math.Round(value * 1e4))

	return pgtype.Numeric{
		Int:   big.NewInt(i),
		Exp:   -4,
		Valid: true,
	}
}

// numericToFloat converts the postgres value to float
func numericToFloat(value pgtype.Numeric) float64 {
	f, _ := new(big.Rat).SetInt(value.Int).Float64()

	if value.Exp > 0 {
		f *= math.Pow10(int(value.Exp))
	} else if value.Exp < 0 {
		f /= math.Pow10(int(-value.Exp))
	}

	return f
}

// timeToTimestampz converts the time value to postgres timestamp
func timeToTimestampz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  t.UTC(),
		Valid: true,
	}
}

// timestampzToTime converts the postgres timestamp value to time
func timestampzToTime(ts pgtype.Timestamptz) time.Time {
	if !ts.Valid {
		return time.Time{}
	}

	return ts.Time
}

// stringArgToText converts the given string value to postgres text
func stringArgToText[T ~string](p *T) pgtype.Text {
	if p == nil {
		return pgtype.Text{
			Valid: false,
		}
	}

	return pgtype.Text{
		String: string(*p),
		Valid:  true,
	}
}
