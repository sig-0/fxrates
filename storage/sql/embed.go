package sql

import "embed"

// SchemaFS contains all SQL migration files under db/sql/schema/
//
//go:embed schema/*.sql
var SchemaFS embed.FS
