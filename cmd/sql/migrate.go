package sql

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/sig-0/fxrates/cmd/env"
	dbpkg "github.com/sig-0/fxrates/storage/sql"
)

// migrateCfg wraps the migrate configuration
type migrateCfg struct {
	rootCfg *sqlCfg
}

// newMigrateCmd creates the migrate command
func newMigrateCmd(rootCfg *sqlCfg) *ffcli.Command {
	cfg := &migrateCfg{
		rootCfg: rootCfg,
	}

	fs := flag.NewFlagSet("migrate", flag.ExitOnError)
	rootCfg.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "migrate",
		ShortUsage: "sql migrate [migration.sql, migration2.sql ...]",
		LongHelp:   "Runs initial DB migrations",
		FlagSet:    fs,
		Exec:       cfg.exec,
		Options: []ff.Option{
			// Allow using ENV variables
			ff.WithEnvVars(),
			ff.WithEnvVarPrefix(env.Prefix),
		},
	}
}

func (c *migrateCfg) exec(ctx context.Context, args []string) error {
	// Make sure some migrations are specified
	if len(args) == 0 {
		return fmt.Errorf("no migration files provided")
	}

	// Load .env
	if err := godotenv.Load(); err != nil {
		return fmt.Errorf("unable to load .env vars")
	}

	dsn := os.Getenv(env.Prefix + env.DBURLSuffix)

	// Open the DB
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}

	// Ping the DB
	if err = db.PingContext(ctx); err != nil {
		return fmt.Errorf("unable to ping DB: %w", err)
	}

	defer func() {
		if err = db.Close(); err != nil {
			fmt.Printf("Unable to gracefully close DB: %s\n", err.Error())
		}
	}()

	for _, name := range args {
		path := fmt.Sprintf("schema/%s", name)

		sqlBytes, err := dbpkg.SchemaFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("unable to read migration %q: %w", name, err)
		}

		fmt.Printf("Running migration %s...\n", name)

		if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("unable to run migration %q: %w", name, err)
		}

		fmt.Printf("Migration %q complete\n", name)
	}

	fmt.Println("All migrations complete!")

	return nil
}
