//go:build !nowasm

package main

import (
	// Cgo-free Wasm database
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"

	// Cgo-free Wasm tagger
	"go.senan.xyz/gonic/tags/taglib"
)

//nolint:gochecknoglobals
var tagReader = taglib.Reader{}
