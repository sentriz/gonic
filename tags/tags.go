package tags

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"go.senan.xyz/wrtag/tags/normtag"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	CanRead(absPath string) bool
	Read(absPath string) (properties Properties, tags Tags, err error)
	ReadCover(absPath string) ([]byte, error)
}

type Tags = map[string][]string

type Properties struct {
	Length   time.Duration
	Bitrate  uint
	HasCover bool
}

const (
	FallbackAlbum  = "Unknown Album"
	FallbackArtist = "Unknown Artist"
	FallbackGenre  = "Unknown Genre"
)

func MustAlbum(p Tags) string {
	if r := normtag.Get(p, normtag.Album); r != "" {
		return r
	}
	return FallbackAlbum
}

func MustArtist(p Tags) string {
	if r := normtag.Get(p, normtag.Artist); r != "" {
		return r
	}
	return FallbackArtist
}

func MustArtists(p Tags) []string {
	if r := normtag.Values(p, normtag.Artists); len(r) > 0 {
		return r
	}
	if r := normtag.Values(p, normtag.Artist); len(r) > 0 {
		return r
	}
	return []string{FallbackArtist}
}

func MustAlbumArtist(p Tags) string {
	if r := normtag.Get(p, normtag.AlbumArtist); r != "" {
		return r
	}
	return MustArtist(p)
}

func MustAlbumArtists(p Tags) []string {
	if r := normtag.Values(p, normtag.AlbumArtists); len(r) > 0 {
		return r
	}
	if r := normtag.Values(p, normtag.AlbumArtist); len(r) > 0 {
		return r
	}
	return []string{MustArtist(p)}
}

func MustGenre(p Tags) string {
	if r := normtag.Get(p, normtag.Genre); r != "" {
		return r
	}
	return FallbackGenre
}

func MustGenres(p Tags) []string {
	if r := normtag.Values(p, normtag.Genres); len(r) > 0 {
		return r
	}
	if r := normtag.Values(p, normtag.Genre); len(r) > 0 {
		return r
	}
	return []string{FallbackGenre}
}

func MustYear(p Tags) int {
	if t := ParseDate(normtag.Get(p, normtag.OriginalDate)); !t.IsZero() {
		return t.Year()
	}
	if t := ParseDate(normtag.Get(p, normtag.Date)); !t.IsZero() {
		return t.Year()
	}
	return 0
}

func ParseFloat(in string) float32 {
	f, _ := strconv.ParseFloat(in, 32)
	return float32(f)
}

func ParseBool(in string) bool {
	b, _ := strconv.ParseBool(in)
	return b
}

func ParseDB(in string) float32 {
	in = strings.ToLower(in)
	in = strings.TrimSuffix(in, " db")
	in = strings.TrimSuffix(in, "db")
	return ParseFloat(in)
}

func ParseInt(in string) int {
	var i int
	_, _ = fmt.Sscanf(in, "%d", &i)
	return i
}

func ParseDate(in string) time.Time {
	t, _ := dateparse.ParseAny(in)
	return t
}
