//go:build nowasm

package main

import (
	// Cgo database
	_ "github.com/mattn/go-sqlite3"

	// Cgo tagger
	"go.senan.xyz/gonic/tags/taglib"
)

//nolint:gochecknoglobals
var tagReader = taglib.Reader{}
