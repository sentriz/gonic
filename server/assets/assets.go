//nolint:gochecknoglobals,golint,stylecheck
package assets

import (
	"embed"
)

//go:embed layouts
var Layouts embed.FS

//go:embed pages
var Pages embed.FS

//go:embed partials
var Partials embed.FS

//go:embed static
var Static embed.FS
