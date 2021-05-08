//nolint:gochecknoglobals,golint,stylecheck
package gonic

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed version.txt
var version string
var Version = fmt.Sprintf("v%s", strings.TrimSuffix(version, "\n"))

const Name = "gonic"
const NameUpper = "GONIC"
