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
) (*types.Page[*types.ExchangeRate], error) {
	cutoff := asOf.UTC()
	base := query.Base.String()

	var (
		target, source, rateType      string
		hasTarget, hasSource, hasType bool
	)

	if query.Target != nil {
		target = query.Target.String()
		hasTarget = true
	}

	if query.Source != nil {
		source = query.Source.String()
		hasSource = true
	}

	if query.RateType != nil {
		rateType = query.RateType.String()
		hasType = true
	}

	type bucket struct {
		target, source, rateType string
	}

	s.mu.RLock()

	bestByBucket := make(map[bucket]types.ExchangeRate)

	for _, v := range s.data {
		if v.Base.String() != base {
			continue
		}

		if hasTarget && v.Target.String() != target {
			continue
		}

		if hasSource && v.Source.String() != source {
			continue
		}

		if hasType && v.RateType.String() != rateType {
			continue
		}

		if v.AsOf.After(cutoff) {
			continue
		}

		b := bucket{
			target:   v.Target.String(),
			source:   v.Source.String(),
			rateType: v.RateType.String(),
		}

		cur, ok := bestByBucket[b]
		if !ok ||
			v.AsOf.After(cur.AsOf) ||
			(v.AsOf.Equal(cur.AsOf) && v.FetchedAt.After(cur.FetchedAt)) {
			bestByBucket[b] = v
		}
	}

	s.mu.RUnlock()

	out := make([]*types.ExchangeRate, 0, len(bestByBucket))
	for _, v := range bestByBucket {
		cp := v
		out = append(out, &cp)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Target != out[j].Target {
			return out[i].Target.String() < out[j].Target.String()
		}

		if out[i].Source != out[j].Source {
			return out[i].Source.String() < out[j].Source.String()
		}

		return out[i].RateType.String() < out[j].RateType.String()
	})

	total := int64(len(out))
	if total == 0 {
		return &types.Page[*types.ExchangeRate]{
			Results: nil,
			Total:   0,
		}, nil
	}

	lim := query.Limit
	if lim == 0 {
		lim = 100
	}

	if lim > 500 {
		lim = 500
	}

	off := query.Offset
	if off > total {
		return &types.Page[*types.ExchangeRate]{
			Results: nil,
			Total:   total,
		}, nil
	}

	start := int(off)
	end := start + int(lim)

	if end > len(out) {
		end = len(out)
	}

	return &types.Page[*types.ExchangeRate]{
		Results: out[start:end],
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
