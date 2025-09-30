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
	Read(absPath string) (properties Properties, tags map[string][]string, err error)
	ReadCover(absPath string) ([]byte, error)
}

type Properties struct {
	Length  time.Duration
	Bitrate uint
}

const (
	FallbackAlbum  = "Unknown Album"
	FallbackArtist = "Unknown Artist"
	FallbackGenre  = "Unknown Genre"
)

func MustAlbum(p map[string][]string) string {
	if r := normtag.Get(p, normtag.Album); r != "" {
		return r
	}
	return FallbackAlbum
}

func MustArtist(p map[string][]string) string {
	if r := normtag.Get(p, normtag.Artist); r != "" {
		return r
	}
	return FallbackArtist
}

func MustArtists(p map[string][]string) []string {
	if r := normtag.Values(p, normtag.Artists); len(r) > 0 {
		return r
	}
	return []string{MustArtist(p)}
}

func MustAlbumArtist(p map[string][]string) string {
	if r := normtag.Get(p, normtag.AlbumArtist); r != "" {
		return r
	}
	return MustArtist(p)
}

func MustAlbumArtists(p map[string][]string) []string {
	if r := normtag.Values(p, normtag.AlbumArtists); len(r) > 0 {
		return r
	}
	return []string{MustAlbumArtist(p)}
}

func MustGenre(p map[string][]string) string {
	if r := normtag.Get(p, normtag.Genre); r != "" {
		return r
	}
	return FallbackGenre
}

func MustGenres(p map[string][]string) []string {
	if r := normtag.Values(p, normtag.Genres); len(r) > 0 {
		return r
	}
	return []string{MustGenre(p)}
}

func MustYear(p map[string][]string) int {
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
