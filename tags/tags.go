package tags

import (
	"errors"
	"fmt"
	"slices"
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
	Length     time.Duration
	Bitrate    uint
	Channels   uint
	SampleRate uint
	BitDepth   uint
	Codec      string
	Container  string
	HasCover   bool
}

//nolint:gochecknoglobals
var (
	Artist = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Artists, KeyCredit: normtag.ArtistsCredit, KeyMusicBrainzID: normtag.MusicBrainzArtistID},
			{Key: normtag.Artist, KeyCredit: normtag.ArtistCredit, KeyMusicBrainzID: normtag.MusicBrainzArtistID},
		},
		Fallback: "Unknown Artist",
	}
	AlbumArtist = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.AlbumArtists, KeyCredit: normtag.AlbumArtistsCredit, KeyMusicBrainzID: normtag.MusicBrainzAlbumArtistID},
			{Key: normtag.AlbumArtist, KeyCredit: normtag.AlbumArtistCredit, KeyMusicBrainzID: normtag.MusicBrainzAlbumArtistID},
			{Key: normtag.Artist, KeyCredit: normtag.ArtistCredit, KeyMusicBrainzID: normtag.MusicBrainzArtistID},
		},
		Fallback: "Unknown Artist",
	}
	Genre = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Genres},
			{Key: normtag.Genre},
		},
	}
	ISRC = &Spec{
		Sources: []Source{
			{Key: normtag.ISRC},
		},
	}

	Remixer = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Remixers, KeyCredit: normtag.RemixersCredit, KeyMusicBrainzID: normtag.MusicBrainzRemixerID},
			{Key: normtag.Remixer, KeyCredit: normtag.RemixerCredit, KeyMusicBrainzID: normtag.MusicBrainzRemixerID},
		},
	}
	Composer = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Composers, KeyCredit: normtag.ComposersCredit, KeyMusicBrainzID: normtag.MusicBrainzComposerID},
			{Key: normtag.Composer, KeyCredit: normtag.ComposerCredit, KeyMusicBrainzID: normtag.MusicBrainzComposerID},
		},
	}
	Lyricist = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Lyricists, KeyCredit: normtag.LyricistsCredit, KeyMusicBrainzID: normtag.MusicBrainzLyricistID},
			{Key: normtag.Lyricist, KeyCredit: normtag.LyricistCredit, KeyMusicBrainzID: normtag.MusicBrainzLyricistID},
		},
	}
	Conductor = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Conductors, KeyCredit: normtag.ConductorsCredit, KeyMusicBrainzID: normtag.MusicBrainzConductorID},
			{Key: normtag.Conductor, KeyCredit: normtag.ConductorCredit, KeyMusicBrainzID: normtag.MusicBrainzConductorID},
		},
	}
	Producer = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Producers, KeyCredit: normtag.ProducersCredit, KeyMusicBrainzID: normtag.MusicBrainzProducerID},
			{Key: normtag.Producer, KeyCredit: normtag.ProducerCredit, KeyMusicBrainzID: normtag.MusicBrainzProducerID},
		},
	}
	Arranger = &Spec{
		Sources: []Source{
			{Multi: true, Key: normtag.Arrangers, KeyCredit: normtag.ArrangersCredit, KeyMusicBrainzID: normtag.MusicBrainzArrangerID},
			{Key: normtag.Arranger, KeyCredit: normtag.ArrangerCredit, KeyMusicBrainzID: normtag.MusicBrainzArrangerID},
		},
	}

	AlbumTitle = &Spec{
		Sources:  []Source{{Key: normtag.Album}},
		Fallback: "Unknown Album",
	}
	TrackTitle = &Spec{
		Sources: []Source{{Key: normtag.Title}},
	}
	Year = &Spec{
		Sources: []Source{{Key: normtag.OriginalDate}, {Key: normtag.Date}},
		Valid:   func(v string) bool { return !ParseDate(v).IsZero() },
	}
)

type Spec struct {
	Sources  []Source
	Fallback string
	Valid    func(string) bool
}

type Source struct {
	Multi                            bool
	Key, KeyCredit, KeyMusicBrainzID string
}

type MultiValueMode uint8

const (
	None MultiValueMode = iota
	Delim
	Multi
)

type MultiValueSetting struct {
	Mode  MultiValueMode
	Delim string
}

func ReadValues(t Tags, spec *Spec, setting MultiValueSetting) []string {
	_, values := read(t, spec, setting)
	return values
}

type Credited struct {
	Value, ValueCredit string
	MusicBrainzID      string
}

func ReadCredits(t Tags, spec *Spec, setting MultiValueSetting) []Credited {
	source, values := read(t, spec, setting)
	credits := readKey(t, source.KeyCredit, setting)
	musicBrainzIDs := readKey(t, source.KeyMusicBrainzID, setting)

	out := make([]Credited, 0, len(values))
	for i, v := range values {
		if v == "" {
			continue
		}
		e := Credited{Value: v}
		if len(credits) == len(values) && credits[i] != v {
			e.ValueCredit = credits[i]
		}
		if len(musicBrainzIDs) == len(values) {
			e.MusicBrainzID = musicBrainzIDs[i]
		}
		out = append(out, e)
	}
	return out
}

func Read(t Tags, spec *Spec) (value, valueCredit string) {
	source, values := read(t, spec, MultiValueSetting{})
	i := slices.IndexFunc(values, func(v string) bool { return valid(spec, v) })
	if i < 0 {
		return "", ""
	}
	value = values[i]
	if credits := readKey(t, source.KeyCredit, MultiValueSetting{}); len(credits) > 0 && credits[0] != value {
		valueCredit = credits[0]
	}
	return value, valueCredit
}

func read(t Tags, spec *Spec, setting MultiValueSetting) (Source, []string) {
	for _, source := range spec.Sources {
		if source.Multi && setting.Mode != Multi {
			continue
		}
		if values := readKey(t, source.Key, setting); slices.ContainsFunc(values, func(v string) bool { return valid(spec, v) }) {
			return source, values
		}
	}
	if spec.Fallback != "" {
		return Source{}, []string{spec.Fallback}
	}
	return Source{}, nil
}

func readKey(t Tags, key string, setting MultiValueSetting) []string {
	if key == "" {
		return nil
	}
	var values []string
	for _, v := range normtag.Values(t, key) {
		var parts = []string{v}
		if setting.Mode == Delim {
			parts = strings.Split(v, setting.Delim)
		}
		for _, p := range parts {
			values = append(values, strings.TrimSpace(p))
		}
	}
	return values
}

func valid(spec *Spec, v string) bool {
	if spec.Valid == nil {
		return v != ""
	}
	return spec.Valid(v)
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
