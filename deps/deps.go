//go:build !nowasm

package deps

import (
	"net/url"

	// Cgo-free Wasm database
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	// Cgo-free Wasm tagger
	"go.senan.xyz/gonic/tags/taglib"
)

//nolint:gochecknoglobals
var TagReader = taglib.Reader{}

// DBDriverOptions returns SQLite DSN options for the ncruces driver
func DBDriverOptions() url.Values {
	return url.Values{
		"_pragma": {
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"foreign_keys(ON)",
		},
	}
}
