// TODO: this package shouldn't really exist. we can usually just attempt our normal filesystem operations
// and handle errors atomically. eg.
// - Safe could instead be try create file, handle error
// - Unique could be try create file, on err create file (1), etc
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlphaNumExpr = regexp.MustCompile("[^a-zA-Z0-9_.]+")

func Safe(filename string) string {
	filename = nonAlphaNumExpr.ReplaceAllString(filename, "")
	return filename
}

// try to find a unqiue file (or dir) name. incrementing like "example (1)"
func Unique(base, filename string) (string, error) {
	return unique(base, filename, 0)
}

func unique(base, filename string, count uint) (string, error) {
	var suffix string
	if count > 0 {
		suffix = fmt.Sprintf(" (%d)", count)
	}
	path := base + suffix
	if filename != "" {
		noExt := strings.TrimSuffix(filename, filepath.Ext(filename))
		path = filepath.Join(base, noExt+suffix+filepath.Ext(filename))
	}
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	return unique(base, filename, count+1)
}

func First(path ...string) (string, error) {
	var err error
	for _, p := range path {
		_, err = os.Stat(p)
		if err == nil {
			return p, nil
		}
	}
	return "", err
}

// HasPrefix checks a path has a prefix, making sure to respect path boundaries. So that /aa & /a does not match, but /a/a & /a does.
func HasPrefix(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, filepath.Clean(prefix)+string(filepath.Separator))
}
