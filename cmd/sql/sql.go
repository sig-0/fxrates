package sql

import (
	"context"
	"flag"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// sqlCfg wraps the sql configuration
type sqlCfg struct{}

// NewSQLCmd creates the sql subcommand
func NewSQLCmd() *ffcli.Command {
	cfg := &sqlCfg{}

	fs := flag.NewFlagSet("sql", flag.ExitOnError)
	cfg.RegisterFlags(fs)

	cmd := &ffcli.Command{
		Name:       "sql",
		ShortUsage: "<subcommand> [flags] [<arg>...]",
		LongHelp:   "Runs fxrates SQL suite",
		FlagSet:    fs,
		Exec: func(_ context.Context, _ []string) error {
			return flag.ErrHelp
		},
	}

	// Add the subcommands
	cmd.Subcommands = []*ffcli.Command{
		newMigrateCmd(cfg),
	}

	return cmd
}

func (c *sqlCfg) RegisterFlags(_ *flag.FlagSet) {
	// nothing for now
}
