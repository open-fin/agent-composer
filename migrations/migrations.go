// Package migrations embeds the SQL schema so the server can apply it on boot. This
// keeps `docker compose up` a single command with no separate migration step.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
