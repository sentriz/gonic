//nolint:gochecknoglobals,golint,stylecheck
package gonic

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var version string
var Version = strings.TrimSpace(version)

const (
	Name      = "gonic"
	NameUpper = "GONIC"
)
