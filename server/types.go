package server

import "github.com/sig-0/fxrates/storage/types"

type SourcesResponse struct {
	Results []types.Source `json:"results"`
}

type CurrenciesResponse struct {
	Results []types.Currency `json:"results"`
}

type ErrorResponse struct {
	Error error `json:"error"`
}
