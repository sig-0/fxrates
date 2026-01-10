package serve

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/sync/errgroup"

	"github.com/sig-0/fxrates/ingest"

	"github.com/sig-0/fxrates/cmd/env"
	"github.com/sig-0/fxrates/server"
	"github.com/sig-0/fxrates/server/config"
	"github.com/sig-0/fxrates/storage/sql"
	gen "github.com/sig-0/fxrates/storage/sql/gen"
)

type serveSQLCfg struct {
	rootCfg *serveCfg
}

// newServeCmd creates the serve command
func newServeSQLCmd(rootCfg *serveCfg) *ffcli.Command {
	cfg := &serveSQLCfg{
		rootCfg: rootCfg,
	}

	fs := flag.NewFlagSet("sql", flag.ExitOnError)
	cfg.rootCfg.registerFlags(fs)

	return &ffcli.Command{
		Name:       "sql",
		ShortUsage: "serve sql [flags]",
		LongHelp:   "Serves the fxrates backend, using an SQL datastore",
		FlagSet:    fs,
		Exec:       cfg.exec,
		Options: []ff.Option{
			// Allow using ENV variables
			ff.WithEnvVars(),
			ff.WithEnvVarPrefix(env.Prefix),
		},
	}
}

// exec executes the server serve command
func (c *serveSQLCfg) exec(ctx context.Context, _ []string) error {
	// Read the server configuration, if any
	if c.rootCfg.configPath != "" {
		serverCfg, err := config.Read(c.rootCfg.configPath)
		if err != nil {
			return fmt.Errorf("unable to read server config, %w", err)
		}

		c.rootCfg.config = serverCfg
	}

	// Create a new logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load .env
	if err := godotenv.Load(); err != nil {
		logger.Warn("unable to load .env file")
	}

	// DB
	dsn := os.Getenv(env.Prefix + env.DBURLSuffix)
	if dsn == "" {
		return fmt.Errorf("missing %s", env.Prefix+env.DBURLSuffix)
	}

	// Open DB connection
	pool, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("unable to open DB connection: %w", err)
	}

	defer func() {
		closeCtx, cancelFn := context.WithTimeout(ctx, time.Second*5)
		defer cancelFn()

		if err = pool.Close(closeCtx); err != nil {
			logger.Error(
				"unable to gracefully close DB connection",
				"err", err,
			)
		}
	}()

	// Check DB reachability
	pingCtx, cancelPing := context.WithTimeout(ctx, time.Second*5)
	defer cancelPing()

	if err = pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("unable to reach DB (ping): %w", err)
	}

	logger.Info("DB ping success")

	// Create an SQL store
	store := sql.NewStorage(gen.New(pool))

	// Create the ingestion service
	orchestrator := ingest.New(store, ingest.WithLogger(logger))
	for _, provider := range defaultProviders() {
		if err = orchestrator.Register(provider); err != nil {
			return fmt.Errorf("unable to register provider: %w", err)
		}
	}

	// Create the server instance
	s, err := server.New(
		store,
		server.WithLogger(logger),
		server.WithConfig(c.rootCfg.config),
	)
	if err != nil {
		return fmt.Errorf("unable to create server, %w", err)
	}

	runCtx, cancelFn := signal.NotifyContext(
		ctx,
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	defer cancelFn()

	group, gCtx := errgroup.WithContext(runCtx)

	// Start the HTTP server
	group.Go(func() error {
		return s.Serve(gCtx)
	})

	// Start the ingestion service
	group.Go(func() error {
		return orchestrator.Start(gCtx)
	})

	return group.Wait()
}
