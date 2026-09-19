// Package migrations embeds the goose migration files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
