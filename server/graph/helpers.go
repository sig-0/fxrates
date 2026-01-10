package graph

import (
	"errors"
	"strings"
	"time"

	"github.com/sig-0/fxrates/server/graph/model"
	"github.com/sig-0/fxrates/storage/types"
)

const (
	defaultLimit = int32(100)
	maxLimit     = int32(500)
)

var (
	errInvalidLimit  = errors.New("invalid limit")
	errInvalidOffset = errors.New("invalid offset")
	errInvalidType   = errors.New("invalid type")
	errInvalidCcy    = errors.New("invalid currency (must be 3 letters A-Z)")
)

func parseAsOf(asOf *model.Time) time.Time {
	if asOf == nil {
		return time.Now().UTC()
	}

	return time.Time(*asOf).UTC()
}

func parseLimitOffset(limit, offset *int32) (int32, int64, error) {
	lim := defaultLimit

	if limit != nil {
		if *limit < 0 {
			return 0, 0, errInvalidLimit
		}

		lim = *limit
	}

	if lim == 0 {
		lim = defaultLimit
	}

	if lim > maxLimit {
		lim = maxLimit
	}

	var off int64

	if offset != nil {
		if *offset < 0 {
			return 0, 0, errInvalidOffset
		}

		off = int64(*offset)
	}

	return lim, off, nil
}

func parseSourceAndType(source *string, rt *model.RateType) (*types.Source, *types.RateType, error) {
	var src *types.Source

	if source != nil {
		if v := strings.TrimSpace(*source); v != "" {
			s := types.Source(v)
			src = &s
		}
	}

	var outRT *types.RateType

	if rt != nil {
		t := types.RateType(strings.ToUpper(string(*rt)))

		switch t {
		case types.RateTypeMID, types.RateTypeBUY, types.RateTypeSELL:
			outRT = &t
		default:
			return nil, nil, errInvalidType
		}
	}

	return src, outRT, nil
}

func parseCurrencySymbol(v string) (types.Currency, error) {
	s := strings.ToUpper(strings.TrimSpace(v))
	if len(s) != 3 {
		return "", errInvalidCcy
	}

	for i := 0; i < 3; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return "", errInvalidCcy
		}
	}

	return types.Currency(s), nil
}

func toModelExchangeRate(in *types.ExchangeRate) *model.ExchangeRate {
	return &model.ExchangeRate{
		AsOf:      model.Time(in.AsOf),
		FetchedAt: model.Time(in.FetchedAt),
		Base:      in.Base.String(),
		Target:    in.Target.String(),
		RateType:  model.RateType(in.RateType.String()),
		Source:    in.Source.String(),
		Rate:      in.Rate,
	}
}

func clampTotalToInt32(total int64) int32 {
	if total <= 0 {
		return 0
	}

	const maxTotal = int64(^uint32(0) >> 1)
	if total > maxTotal {
		return int32(maxTotal)
	}

	return int32(total)
}
