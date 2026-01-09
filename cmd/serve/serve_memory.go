package serve

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
	"golang.org/x/sync/errgroup"

	"github.com/sig-0/fxrates/cmd/env"
	"github.com/sig-0/fxrates/server"
	"github.com/sig-0/fxrates/server/config"
	"github.com/sig-0/fxrates/storage/memory"
)

type serveMemoryCfg struct {
	rootCfg *serveCfg
}

// newServeMemoryCmd creates the serve memory command.
func newServeMemoryCmd(rootCfg *serveCfg) *ffcli.Command {
	cfg := &serveMemoryCfg{
		rootCfg: rootCfg,
	}

	fs := flag.NewFlagSet("memory", flag.ExitOnError)
	cfg.rootCfg.registerFlags(fs)

	return &ffcli.Command{
		Name:       "memory",
		ShortUsage: "serve memory [flags]",
		LongHelp:   "Serves the fxrates backend, using an in-memory datastore",
		FlagSet:    fs,
		Exec:       cfg.exec,
		Options: []ff.Option{
			ff.WithEnvVars(),
			ff.WithEnvVarPrefix(env.Prefix),
		},
	}
}

func (c *serveMemoryCfg) exec(ctx context.Context, _ []string) error {
	// Read the server configuration, if any
	if c.rootCfg.configPath != "" {
		serverCfg, err := config.Read(c.rootCfg.configPath)
		if err != nil {
			return fmt.Errorf("unable to read server config, %w", err)
		}

		c.rootCfg.config = serverCfg
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Load .env
	if err := godotenv.Load(); err != nil {
		logger.Warn("unable to load .env file")
	}

	// Create an in-memory store
	store := memory.NewStorage()

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

	group.Go(func() error {
		return s.Serve(gCtx)
	})

	return group.Wait()
}
