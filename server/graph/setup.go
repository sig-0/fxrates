package graph

import (
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/vektah/gqlparser/v2/ast"

	"github.com/sig-0/fxrates/storage"
)

// Setup sets up the GraphQL server on the given mux
func Setup(storage storage.Storage, m *chi.Mux) *chi.Mux {
	srv := handler.New(NewExecutableSchema(
		Config{
			Resolvers: NewResolver(storage),
		},
	))

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	m.Handle("/graphql/query", srv)
	m.Handle("/graphql", playground.Handler("fxrates: GraphQL playground", "/graphql/query"))

	// TODO add examples

	return m
}
