package serve

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/sig-0/fxrates/cmd/env"
	"github.com/sig-0/fxrates/server/config"
)

// serveCfg wraps the serve configuration
type serveCfg struct {
	config *config.Config

	configPath string
}

// NewServeCmd creates the serve subcommand
func NewServeCmd() *ffcli.Command {
	cfg := &serveCfg{
		config: config.DefaultConfig(),
	}

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	cfg.registerFlags(fs)

	cmd := &ffcli.Command{
		Name:       "serve",
		ShortUsage: "serve <subcommand> [flags]",
		LongHelp:   "Serves the fxrates backend",
		FlagSet:    fs,
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
		Options: []ff.Option{
			// Allow using ENV variables
			ff.WithEnvVars(),
			ff.WithEnvVarPrefix(env.Prefix),
		},
	}

	cmd.Subcommands = []*ffcli.Command{
		newServeSQLCmd(cfg),
		newServeMemoryCmd(cfg),
	}

	return cmd
}

func (c *serveCfg) registerFlags(fs *flag.FlagSet) {
	fs.StringVar(
		&c.config.ListenAddress,
		"listen",
		config.DefaultListenAddress,
		"the IP:PORT URL for the server",
	)

	fs.StringVar(
		&c.configPath,
		"config",
		"",
		"the path to the server TOML configuration, if any",
	)
}
