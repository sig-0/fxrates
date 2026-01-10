package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v3"
	"github.com/rs/cors"
	"golang.org/x/sync/errgroup"

	"github.com/sig-0/fxrates/storage"

	"github.com/sig-0/fxrates/server/config"
)

// RoutesFn is a callback that receives a router for registering routes
type RoutesFn func(router chi.Router)

var noopLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type Server struct {
	logger *slog.Logger
	config *config.Config

	storage storage.Storage

	mux *chi.Mux
}

// New creates a new server instance
func New(storage storage.Storage, opts ...Option) (*Server, error) {
	s := &Server{
		logger:  noopLogger,
		storage: storage,
		config:  config.DefaultConfig(),
		mux:     chi.NewMux(),
	}

	// Apply the options
	for _, opt := range opts {
		opt(s)
	}

	// Validate the configuration
	if err := config.ValidateConfig(s.config); err != nil {
		return nil, fmt.Errorf("invalid configuration, %w", err)
	}

	// Set up the CORS middleware
	if s.config.CORSConfig != nil {
		corsMiddleware := cors.New(cors.Options{
			AllowedOrigins: s.config.CORSConfig.AllowedOrigins,
			AllowedMethods: s.config.CORSConfig.AllowedMethods,
			AllowedHeaders: s.config.CORSConfig.AllowedHeaders,
		})

		s.mux.Use(corsMiddleware.Handler)
	}

	s.mux.Use(httplog.RequestLogger(s.logger, &httplog.Options{
		Level:         slog.LevelInfo,     // TODO expose this in the config
		Schema:        httplog.SchemaOTEL, // TODO expose this in the config
		RecoverPanics: true,
		Skip: func(_ *http.Request, respStatus int) bool {
			return respStatus == 404 || respStatus == 405 // TODO skip health pings
		},
	}))

	// Register the health check handler
	s.mux.Get("/health", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	// TODO add standard handler endpoints

	return s, nil
}

// Routes calls fn with the server mux so callers can add endpoints
func (s *Server) Routes(fn RoutesFn) {
	if fn == nil {
		return
	}

	fn(s.mux)
}

// Serve serves the fxrates service
func (s *Server) Serve(ctx context.Context) error {
	server := &http.Server{
		Addr:              s.config.ListenAddress,
		Handler:           s.mux,
		ReadHeaderTimeout: 60 * time.Second,
	}

	group, gCtx := errgroup.WithContext(ctx)

	group.Go(func() error {
		defer s.logger.Info("server shut down")

		ln, err := net.Listen("tcp", server.Addr)
		if err != nil {
			return err
		}

		s.logger.Info(
			fmt.Sprintf(
				"server started at %s",
				ln.Addr().String(),
			),
		)

		if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}

		return nil
	})

	group.Go(func() error {
		<-gCtx.Done()

		s.logger.Info("server to be shutdown")

		wsCtx, cancel := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel()

		return server.Shutdown(wsCtx)
	})

	return group.Wait()
}
