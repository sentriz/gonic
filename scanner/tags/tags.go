package tags

import (
	"errors"
	"strconv"
	"strings"

	"github.com/psyton/audiotags"
)

var _ MetaDataProvider = (*Tagger)(nil)

type TagReader struct{}

var ErrorUnsupportedMedia = errors.New("unsupported media")

const UnknownArtist = "Unknown Artist"
const UnknownAlbum = "Unknown Album"
const UnknownGenre = "Unknown Genre"

func (*TagReader) Read(abspath string) (MetaDataProvider, error) {
	f, err := audiotags.Open(abspath)
	if err != nil {
		return nil, err
	}
	if !f.HasMedia() {
		return nil, ErrorUnsupportedMedia
	}

	return &Tagger{f.ReadTags(), f.ReadAudioProperties()}, nil
}

type Tagger struct {
	raw   map[string]string
	props *audiotags.AudioProperties
}

func (t *Tagger) first(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(t.raw[key]); v != "" {
			return v
		}
	}
	return ""
}

func (t *Tagger) firstInt(sep string, keys ...string) int {
	for _, key := range keys {
		if v := intSep(t.raw[key], sep); v > 0 {
			return v
		}
	}
	return 0
}

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

func (t *Tagger) Title() string         { return t.first("title") }
func (t *Tagger) BrainzID() string      { return t.first("musicbrainz_trackid") } // musicbrainz recording ID
func (t *Tagger) Artist() string        { return t.first("artist") }
func (t *Tagger) Album() string         { return t.first("album") }
func (t *Tagger) AlbumArtist() string   { return t.first("albumartist", "album artist") }
func (t *Tagger) AlbumBrainzID() string { return t.first("musicbrainz_albumid") } // musicbrainz release ID
func (t *Tagger) Genre() string         { return t.first("genre") }
func (t *Tagger) TrackNumber() int      { return t.firstInt("/" /* eg. 5/12 */, "tracknumber") }
func (t *Tagger) DiscNumber() int       { return t.firstInt("/" /* eg. 1/2  */, "discnumber") }
func (t *Tagger) Length() int           { return t.props.LengthMs }
func (t *Tagger) Bitrate() int          { return t.props.Bitrate }
func (t *Tagger) Year() int             { return t.firstInt("-", "originaldate", "date", "year") }
func (t *Tagger) CueSheet() string      { return t.first("cuesheet") }

func (t *Tagger) SomeAlbum() string  { return First(UnknownAlbum, t.Album()) }
func (t *Tagger) SomeArtist() string { return First(UnknownArtist, t.Artist()) }
func (t *Tagger) SomeAlbumArtist() string {
	return First(UnknownArtist, t.AlbumArtist(), t.Artist())
}
func (t *Tagger) SomeGenre() string { return First(UnknownGenre, t.Genre()) }

type Reader interface {
	Read(absPath string) (MetaDataProvider, error)
}

type EmbeddedCueProvider interface {
	CueSheet() string
}

type MetaDataProvider interface {
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

func First(or string, strs ...string) string {
	for _, str := range strs {
		if str != "" {
			return str
		}
	}
	return or
}
