// Package migrations embeds the SQL migrations so the API binary can run them at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
