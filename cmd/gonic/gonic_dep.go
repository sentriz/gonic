//go:build !nowasm

package main

import (
	// CGo-free Wasm tagger
	"go.senan.xyz/gonic/tags/wrtag"
)

//nolint:gochecknoglobals
var tagReader = wrtag.Reader{}
