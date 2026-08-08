// Package migrations embeds the SQL migration files so they ship inside
// the compiled binary (no separate file mount needed in the container).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
