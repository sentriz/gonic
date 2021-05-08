//nolint:gochecknoglobals,golint,stylecheck
package gonic

import (
	_ "embed"
)

//go:embed version.txt
var Version string

const Name = "gonic"
const NameUpper = "GONIC"
