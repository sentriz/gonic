//go:build nowasm

package main

import (

	// Cgo tagger
	"go.senan.xyz/gonic/tags/taglib"
)

//nolint:gochecknoglobals
var tagReader = taglib.Reader{}
