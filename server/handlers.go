package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/sig-0/fxrates/storage/types"
)

const (
	defaultLimit = int32(100)
	maxLimit     = int32(500)
)

var (
	errUnableToFetchRates      = errors.New("unable to fetch rates")
	errUnableToFetchCurrencies = errors.New("unable to fetch currencies")
	errUnableToFetchSources    = errors.New("unable to fetch sources")

	errInvalidLimit  = errors.New("invalid limit")
	errInvalidOffset = errors.New("invalid offset")
	errInvalidType   = errors.New("invalid type")
)

func (s *Server) RatesForPair(w http.ResponseWriter, r *http.Request) {
	var (
		baseParam   = chi.URLParam(r, "base")
		targetParam = chi.URLParam(r, "target")

		asOfParam   = r.URL.Query().Get("as_of")
		limitParam  = r.URL.Query().Get("limit")
		offsetParam = r.URL.Query().Get("offset")

		sourceParam = r.URL.Query().Get("source")
		typeParam   = r.URL.Query().Get("type")
	)

	// Parse the base currency
	base, err := parseCurrencySymbol(baseParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the target currency
	target, err := parseCurrencySymbol(targetParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the effective date (defaults to now)
	asOf, err := parseAsOf(asOfParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the pagination settings
	limit, offset, err := parseLimitOffset(limitParam, offsetParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the source and rate type (optional)
	source, rateType, err := parseSourceAndType(sourceParam, typeParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	q := &types.RateQuery{
		Base:     base,
		Target:   &target,
		Source:   source,
		RateType: rateType,
		Limit:    limit,
		Offset:   offset,
	}

	page, err := s.storage.RateAsOf(r.Context(), q, asOf)
	if err != nil {
		s.logger.Debug(
			"unable to fetch rates",
			"err", err,
		)

		writeError(
			w,
			http.StatusInternalServerError,
			errUnableToFetchRates,
		)

		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (s *Server) RatesForBase(w http.ResponseWriter, r *http.Request) {
	var (
		baseParam = chi.URLParam(r, "base")

		asOfParam   = r.URL.Query().Get("as_of")
		limitParam  = r.URL.Query().Get("limit")
		offsetParam = r.URL.Query().Get("offset")

		sourceParam = r.URL.Query().Get("source")
		typeParam   = r.URL.Query().Get("type")
	)

	// Parse the base currency
	base, err := parseCurrencySymbol(baseParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the effective date
	asOf, err := parseAsOf(asOfParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the pagination settings
	limit, offset, err := parseLimitOffset(limitParam, offsetParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	// Parse the source and rate type (optional)
	source, rateType, err := parseSourceAndType(sourceParam, typeParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)

		return
	}

	q := &types.RateQuery{
		Base:     base,
		Target:   nil,
		Source:   source,
		RateType: rateType,
		Limit:    limit,
		Offset:   offset,
	}

	page, err := s.storage.RateAsOf(r.Context(), q, asOf)
	if err != nil {
		s.logger.Debug(
			"unable to fetch rates",
			"err", err,
		)

		writeError(
			w,
			http.StatusInternalServerError,
			errUnableToFetchRates,
		)

		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (s *Server) Sources(w http.ResponseWriter, r *http.Request) {
	items, err := s.storage.ListSources(r.Context())
	if err != nil {
		s.logger.Debug(
			"unable to fetch sources",
			"err", err,
		)

		writeError(
			w,
			http.StatusInternalServerError,
			errUnableToFetchSources,
		)

		return
	}

	resp := &SourcesResponse{
		Results: items,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) Currencies(w http.ResponseWriter, r *http.Request) {
	items, err := s.storage.ListCurrencies(r.Context())
	if err != nil {
		s.logger.Debug(
			"unable to fetch currencies",
			"err", err,
		)

		writeError(
			w,
			http.StatusInternalServerError,
			errUnableToFetchCurrencies,
		)

		return
	}

	resp := &CurrenciesResponse{
		Results: items,
	}

	writeJSON(w, http.StatusOK, resp)
}

func parseAsOf(asOfRaw string) (time.Time, error) {
	v := strings.TrimSpace(asOfRaw)
	if v == "" {
		return time.Now().UTC(), nil // default is now
	}

	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, errors.New("invalid as_of (must be RFC3339 UTC)")
	}

	return t.UTC(), nil
}

func parseLimitOffset(limitRaw, offsetRaw string) (int32, int64, error) {
	limit := defaultLimit

	if v := strings.TrimSpace(limitRaw); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, 0, errInvalidLimit
		}

		limit = int32(n) //nolint:gosec // Fine to clamp
	}

	if limit == 0 {
		limit = defaultLimit
	}

	if limit > maxLimit {
		limit = maxLimit
	}

	var offset int64

	if v := strings.TrimSpace(offsetRaw); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, 0, errInvalidOffset
		}

		offset = n
	}

	return limit, offset, nil
}

func parseSourceAndType(sourceRaw, typeRaw string) (*types.Source, *types.RateType, error) {
	var src *types.Source

	if v := strings.TrimSpace(sourceRaw); v != "" {
		s := types.Source(v)

		src = &s
	}

	var rt *types.RateType

	if v := strings.TrimSpace(typeRaw); v != "" {
		t := types.RateType(strings.ToUpper(v))

		switch t {
		case types.RateTypeMID, types.RateTypeBUY, types.RateTypeSELL:
			rt = &t
		default:
			return nil, nil, errInvalidType
		}
	}

	return src, rt, nil
}

func parseCurrencySymbol(v string) (types.Currency, error) {
	// TODO use regex?
	s := strings.ToUpper(strings.TrimSpace(v))
	if len(s) != 3 {
		return "", errors.New("invalid currency (must be 3 letters)")
	}

	for i := 0; i < 3; i++ {
		if s[i] < 'A' || s[i] > 'Z' {
			return "", errors.New("invalid currency (must be A-Z)")
		}
	}

	return types.Currency(s), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(v) //nolint:errcheck // Fine to ignore
}

func writeError(w http.ResponseWriter, status int, err error) {
	resp := &ErrorResponse{
		Error: err,
	}

	writeJSON(w, status, resp)
}
