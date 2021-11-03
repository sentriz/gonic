package tags

import (
	"strconv"
	"strings"

	"github.com/nicksellen/audiotags"
)

type TagReader struct{}

func (*TagReader) Read(abspath string) (Parser, error) {
	raw, props, err := audiotags.Read(abspath)
	return &Tagger{raw, props}, err
}

type Tagger struct {
	raw   map[string]string
	props *audiotags.AudioProperties
}

func (t *Tagger) first(keys ...string) string {
	for _, key := range keys {
		if val, ok := t.raw[key]; ok {
			return val
		}
	}
	return ""
}

func (t *Tagger) Title() string         { return t.first("title") }
func (t *Tagger) BrainzID() string      { return t.first("musicbrainz_trackid") }
func (t *Tagger) Artist() string        { return t.first("artist") }
func (t *Tagger) Album() string         { return t.first("album") }
func (t *Tagger) AlbumArtist() string   { return t.first("albumartist", "album artist") }
func (t *Tagger) AlbumBrainzID() string { return t.first("musicbrainz_albumid") }
func (t *Tagger) Genre() string         { return t.first("genre") }
func (t *Tagger) TrackNumber() int      { return intSep(t.first("tracknumber"), "/") } // eg. 5/12
func (t *Tagger) DiscNumber() int       { return intSep(t.first("discnumber"), "/") }  // eg. 1/2
func (t *Tagger) Length() int           { return t.props.Length }
func (t *Tagger) Bitrate() int          { return t.props.Bitrate }
func (t *Tagger) Year() int             { return intSep(t.first("originaldate", "date", "year"), "-") }

func (t *Tagger) SomeAlbum() string  { return first("Unknown Album", t.Album()) }
func (t *Tagger) SomeArtist() string { return first("Unknown Artist", t.Artist()) }
func (t *Tagger) SomeAlbumArtist() string {
	return first("Unknown Artist", t.AlbumArtist(), t.Artist())
}
func (t *Tagger) SomeGenre() string { return first("Unknown Genre", t.Genre()) }

func first(or string, strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return or
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

type Reader interface {
	Read(abspath string) (Parser, error)
}

type Parser interface {
	Title() string
	BrainzID() string
	Artist() string
	Album() string
	AlbumArtist() string
	AlbumBrainzID() string
	Genre() string
	TrackNumber() int
	DiscNumber() int
	Length() int
	Bitrate() int
	Year() int

	SomeAlbum() string
	SomeArtist() string
	SomeAlbumArtist() string
	SomeGenre() string
}
