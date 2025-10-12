//go:build nowasm

package main

import (
	// Cgo database
	_ "github.com/mattn/go-sqlite3"

	// Cgo tagger
	"go.senan.xyz/gonic/tags/ffprobe"
)

//nolint:gochecknoglobals
var tagReader = ffprobe.Reader{}
