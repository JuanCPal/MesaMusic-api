package migrations

import "embed"

// FS contiene las migraciones SQL embebidas en el binario.
//
//go:embed *.sql
var FS embed.FS
