// Package migrations holds Bothan's embedded, versioned SQL migrations.
//
// Files are named NNNN_description.sql and applied in lexical order by the
// store's migration runner. Each file is applied at most once and recorded in
// the schema_migrations table.
package migrations

import "embed"

// FS contains all .sql migration files, embedded into the binary.
//
//go:embed *.sql
var FS embed.FS
