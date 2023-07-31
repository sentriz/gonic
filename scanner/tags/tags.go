package tags

import (
	"strconv"
	"strings"

	"github.com/sentriz/audiotags"
)

type TagReader struct{}

func (*TagReader) Read(abspath string) (Parser, error) {
	raw, props, err := audiotags.Read(abspath)
	return &Tagger{raw, props}, err
}

type Tagger struct {
	raw   map[string][]string
	props *audiotags.AudioProperties
}

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

func (t *Tagger) Title() string          { return first(find(t.raw, "title")) }
func (t *Tagger) BrainzID() string       { return first(find(t.raw, "musicbrainz_trackid")) } // musicbrainz recording ID
func (t *Tagger) Artist() string         { return first(find(t.raw, "artist")) }
func (t *Tagger) Album() string          { return first(find(t.raw, "album")) }
func (t *Tagger) AlbumArtist() string    { return first(find(t.raw, "albumartist", "album artist")) }
func (t *Tagger) AlbumArtists() []string { return find(t.raw, "albumartists", "album_artists") }
func (t *Tagger) AlbumBrainzID() string  { return first(find(t.raw, "musicbrainz_albumid")) } // musicbrainz release ID
func (t *Tagger) Genre() string          { return first(find(t.raw, "genre")) }

func (t *Tagger) TrackNumber() int {
	return intSep("/" /* eg. 5/12 */, first(find(t.raw, "tracknumber")))
}
func (t *Tagger) DiscNumber() int {
	return intSep("/" /* eg. 1/2  */, first(find(t.raw, "discnumber")))
}
func (t *Tagger) Year() int {
	return intSep("-" /* 2023-12-01 */, first(find(t.raw, "originaldate", "date", "year")))
}

func (t *Tagger) Length() int  { return t.props.Length }
func (t *Tagger) Bitrate() int { return t.props.Bitrate }

type Reader interface {
	Read(abspath string) (Parser, error)
}

type Parser interface {
	Title() string
	BrainzID() string
	Artist() string
	Album() string
	AlbumArtist() string
	AlbumArtists() []string
	AlbumBrainzID() string
	Genre() string
	TrackNumber() int
	DiscNumber() int
	Length() int
	Bitrate() int
	Year() int
}

func fallback(or string, strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return or
}

func first[T comparable](is []T) T {
	var z T
	for _, i := range is {
		if i != z {
			return i
		}
	}
	return z
}

func find(m map[string][]string, keys ...string) []string {
	for _, k := range keys {
		if r := filterStr(m[k]); len(r) > 0 {
			return r
		}
	}
	return nil
}

func filterStr(ss []string) []string {
	var r []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			r = append(r, s)
		}
	}
	return r
}

func intSep(in, sep string) int {
	if in == "" {
		return 0
	}
	start := strings.SplitN(in, sep, 2)[0]
	out, err := strconv.Atoi(start)
	if err != nil {
		return 0
	}
	return out
}

func MustAlbum(p Parser) string {
	if r := p.Album(); r != "" {
		return r
	}
	return "Unknown Album"
}

func MustArtist(p Parser) string {
	if r := p.Artist(); r != "" {
		return r
	}
	return "Unknown Artist"
}

func MustAlbumArtist(p Parser) string {
	if r := p.AlbumArtist(); r != "" {
		return r
	}
	if r := p.Artist(); r != "" {
		return r
	}
	return "Unknown Artist"
}

func MustAlbumArtists(p Parser) []string {
	if r := p.AlbumArtists(); len(r) > 0 {
		return r
	}
	if r := p.AlbumArtist(); r != "" {
		return []string{r}
	}
	if r := p.Artist(); r != "" {
		return []string{r}
	}
	return []string{"Unknown Artist"}
}

func MustGenre(p Parser) string {
	if r := p.Genre(); r != "" {
		return r
	}
	return "Unknown Genre"
}
