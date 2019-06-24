// +build !embed

package server

import "github.com/omeid/go-resources/live"

var (
	assets = live.Dir("./assets_src")
)
