package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/sig-0/fxrates/storage/types"
)

type key struct {
	base, target, source, rateType string
	asOf                           int64 // unix nanos
}

type Storage struct {
	data map[key]types.ExchangeRate

	mu sync.RWMutex
}

func NewStorage() *Storage {
	return &Storage{
		data: make(map[key]types.ExchangeRate),
	}
}

func (s *Storage) SaveExchangeRate(_ context.Context, r *types.ExchangeRate) error {
	k := key{
		base:     r.Base.String(),
		target:   r.Target.String(),
		source:   r.Source.String(),
		rateType: r.RateType.String(),
		asOf:     r.AsOf.UTC().UnixNano(),
	}

	elem := *r
	elem.AsOf = elem.AsOf.UTC()
	elem.FetchedAt = elem.FetchedAt.UTC()

	s.mu.Lock()
	s.data[k] = elem // key is unique
	s.mu.Unlock()

	return nil
}

func (s *Storage) RateAsOf(
	_ context.Context,
	query *types.RateQuery,
	asOf time.Time,
) (*types.ExchangeRate, error) {
	var (
		cutoff   = asOf.UTC()
		base     = query.Base.String()
		target   = query.Target.String()
		source   = query.Source.String()
		rateType = query.RateType.String()
	)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var best *types.ExchangeRate

	for _, v := range s.data {
		if v.Base.String() != base || v.Target.String() != target ||
			v.Source.String() != source || v.RateType.String() != rateType {
			continue
		}

		if v.AsOf.After(cutoff) {
			continue
		}

		if best == nil ||
			v.AsOf.After(best.AsOf) ||
			(v.AsOf.Equal(best.AsOf) && v.FetchedAt.After(best.FetchedAt)) {
			tmp := v
			best = &tmp
		}
	}

	if best == nil {
		return nil, nil //nolint:nilnil // valid case
	}

	return best, nil
}

func (s *Storage) RatesInRange(
	_ context.Context,
	query *types.RateQuery,
	from time.Time,
	to time.Time,
	limit int32,
	offset int64,
) (*types.Page[*types.ExchangeRate], error) {
	var (
		start    = from.UTC()
		end      = to.UTC()
		base     = query.Base.String()
		target   = query.Target.String()
		source   = query.Source.String()
		rateType = query.RateType.String()
	)

	s.mu.RLock()

	matches := make([]types.ExchangeRate, 0)

	for _, v := range s.data {
		if v.Base.String() != base || v.Target.String() != target ||
			v.Source.String() != source || v.RateType.String() != rateType {
			continue
		}

		if v.AsOf.Before(start) || v.AsOf.After(end) {
			continue
		}

		matches = append(matches, v)
	}

	s.mu.RUnlock()

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].AsOf.Equal(matches[j].AsOf) {
			return matches[i].FetchedAt.Before(matches[j].FetchedAt)
		}

		return matches[i].AsOf.Before(matches[j].AsOf)
	})

	total := int64(len(matches))

	if total == 0 {
		return &types.Page[*types.ExchangeRate]{
			Results: nil,
			Total:   0,
		}, nil
	}

	if offset >= total {
		return &types.Page[*types.ExchangeRate]{
			Results: nil,
			Total:   total,
		}, nil
	}

	var (
		startIdx = int(offset)
		endIdx   = len(matches)
	)

	if limit > 0 {
		if cand := startIdx + int(limit); cand < endIdx {
			endIdx = cand
		}
	}

	items := make([]*types.ExchangeRate, 0, endIdx-startIdx)

	for i := startIdx; i < endIdx; i++ {
		cp := matches[i]
		items = append(items, &cp)
	}

	return &types.Page[*types.ExchangeRate]{
		Results: items,
		Total:   total,
	}, nil
}

func (s *Storage) ListSources(_ context.Context) ([]types.Source, error) {
	s.mu.RLock()

	seen := make(map[string]struct{})

	for k := range s.data {
		seen[k.source] = struct{}{}
	}

	s.mu.RUnlock()

	out := make([]types.Source, 0, len(seen))

	for v := range seen {
		out = append(out, types.Source(v))
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})

	return out, nil
}

func (s *Storage) ListCurrencies(_ context.Context) ([]types.Currency, error) {
	s.mu.RLock()

	seen := make(map[string]struct{})

	for k := range s.data {
		seen[k.base] = struct{}{}
		seen[k.target] = struct{}{}
	}

	s.mu.RUnlock()

	out := make([]types.Currency, 0, len(seen))

	for v := range seen {
		out = append(out, types.Currency(v))
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].String() < out[j].String()
	})

	return out, nil
}
