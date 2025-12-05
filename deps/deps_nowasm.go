//go:build nowasm

package deps

import (
	"net/url"

	// Cgo database
	_ "github.com/mattn/go-sqlite3"

	// Cgo tagger
	"go.senan.xyz/gonic/tags/ffprobe"
)

//nolint:gochecknoglobals
var TagReader = ffprobe.Reader{}

// DSNOptions returns SQLite DSN options for the mattn driver
func DBDriverOptions() url.Values {
	return url.Values{
		"_busy_timeout": {"30000"},
		"_journal_mode": {"WAL"},
		"_foreign_keys": {"true"},
	}
}
