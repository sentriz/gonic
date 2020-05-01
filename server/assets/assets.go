package assets

import (
	"strings"
)

// PrefixDo runs a given callback for every path in our assets with
// the given prefix
func PrefixDo(pre string, cb func(path string, asset *EmbeddedAsset)) {
	for path, asset := range Bytes {
		if strings.HasPrefix(path, pre) {
			cb(path, asset)
		}
	}
}
