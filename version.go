package gonic

import (
	_ "embed"
	"strings"
)

//nolint:gochecknoglobals
var (
	//go:embed version.txt
	version string

	Version = strings.TrimSpace(version)
)

const (
	Name      = "gonic"
	NameUpper = "GONIC"
)
