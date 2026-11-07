//go:build !nowasm && ffprobe

// INFO: build with `go build -tags ffprobe -o .\gonic-ffprobe .\cmd\...`

package deps

import (
	"net/url"

	"go.senan.xyz/gonic/tags"
	"go.senan.xyz/gonic/tags/taglib"

	// Cgo-free Wasm database
	_ "github.com/ncruces/go-sqlite3/driver"

	// Cgo-free Wasm tagger
	"go.senan.xyz/gonic/tags/ffprobe"
)

type combinedReader struct {
	taglib  tags.Reader
	ffprobe tags.Reader
}

func (r combinedReader) reader(absPath string) (tags.Reader, error) {
	if r.taglib.CanRead(absPath) {
		return r.taglib, nil
	}
	if r.ffprobe.CanRead(absPath) {
		return r.ffprobe, nil
	}
	return nil, tags.ErrUnsupported
}

func (r combinedReader) CanRead(absPath string) bool {
	_, err := r.reader(absPath)
	return err == nil
}

func (r combinedReader) Read(absPath string) (tags.Properties, tags.Tags, error) {
	reader, err := r.reader(absPath)
	if err != nil {
		return tags.Properties{}, nil, err
	}
	return reader.Read(absPath)
}

func (r combinedReader) ReadCover(absPath string) ([]byte, error) {
	reader, err := r.reader(absPath)
	if err != nil {
		return nil, err
	}
	return reader.ReadCover(absPath)
}

//nolint:gochecknoglobals
var TagReader = combinedReader{
	taglib:  taglib.Reader{},
	ffprobe: ffprobe.Reader{},
}

// DBDriverOptions returns SQLite DSN options for the ncruces driver
func DBDriverOptions() url.Values {
	return url.Values{
		"_pragma": {
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"foreign_keys(ON)",
			"cache_size(-32000)",
		},
	}
}
