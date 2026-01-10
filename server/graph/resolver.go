package graph

import "github.com/sig-0/fxrates/storage"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require
// here.

type Resolver struct {
	Storage storage.Storage
}

func NewResolver(s storage.Storage) *Resolver {
	return &Resolver{
		Storage: s,
	}
}
