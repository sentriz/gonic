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
	Length   time.Duration
	Bitrate  uint
	HasCover bool
}

//nolint:gochecknoglobals
var (
	Artist = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Artists}, Key: []string{normtag.Artist}},
		KeysCredit: Keys{MultiKey: []string{normtag.ArtistsCredit}, Key: []string{normtag.ArtistCredit}},
		Fallback:   "Unknown Artist",
	}
	AlbumArtist = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.AlbumArtists}, Key: []string{normtag.AlbumArtist, normtag.Artist}},
		KeysCredit: Keys{MultiKey: []string{normtag.AlbumArtistsCredit}, Key: []string{normtag.AlbumArtistCredit}},
		Fallback:   "Unknown Artist",
	}
	Genre = &Spec{
		Keys:     Keys{MultiKey: []string{normtag.Genres}, Key: []string{normtag.Genre}},
		Fallback: "Unknown Genre",
	}
	ISRC = &Spec{
		Keys: Keys{MultiKey: []string{normtag.ISRC}, Key: []string{normtag.ISRC}},
	}

	Remixer = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Remixers}, Key: []string{normtag.Remixer}},
		KeysCredit: Keys{MultiKey: []string{normtag.RemixersCredit}, Key: []string{normtag.RemixerCredit}},
	}
	Composer = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Composers}, Key: []string{normtag.Composer}},
		KeysCredit: Keys{MultiKey: []string{normtag.ComposersCredit}, Key: []string{normtag.ComposerCredit}},
	}
	Lyricist = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Lyricists}, Key: []string{normtag.Lyricist}},
		KeysCredit: Keys{MultiKey: []string{normtag.LyricistsCredit}, Key: []string{normtag.LyricistCredit}},
	}
	Conductor = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Conductors}, Key: []string{normtag.Conductor}},
		KeysCredit: Keys{MultiKey: []string{normtag.ConductorsCredit}, Key: []string{normtag.ConductorCredit}},
	}
	Producer = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Producers}, Key: []string{normtag.Producer}},
		KeysCredit: Keys{MultiKey: []string{normtag.ProducersCredit}, Key: []string{normtag.ProducerCredit}},
	}
	Arranger = &Spec{
		Keys:       Keys{MultiKey: []string{normtag.Arrangers}, Key: []string{normtag.Arranger}},
		KeysCredit: Keys{MultiKey: []string{normtag.ArrangersCredit}, Key: []string{normtag.ArrangerCredit}},
	}

	AlbumTitle = &Spec{
		Keys:     Keys{Key: []string{normtag.Album}},
		Fallback: "Unknown Album",
	}
	TrackTitle = &Spec{
		Keys: Keys{Key: []string{normtag.Title}},
	}
	Year = &Spec{
		Keys:  Keys{Key: []string{normtag.OriginalDate, normtag.Date}},
		Valid: func(v string) bool { return !ParseDate(v).IsZero() },
	}
)

type Spec struct {
	Keys, KeysCredit Keys
	Fallback         string
	Valid            func(string) bool
}

type Keys struct {
	MultiKey, Key []string
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

func ReadMulti(t Tags, spec *Spec, settings map[*Spec]MultiValueSetting) (values, valuesCredit []string) {
	setting, ok := settings[spec]
	if !ok {
		setting = MultiValueSetting{Mode: Multi}
	}

	values = read(t, spec.Keys, setting, spec.Valid)
	if len(values) == 0 && spec.Fallback != "" {
		values = []string{spec.Fallback}
	}

	valuesCredit = read(t, spec.KeysCredit, setting, nil)

	return values, valuesCredit
}

func Read(t Tags, spec *Spec) (value, valueCredit string) {
	if parts := read(t, spec.Keys, MultiValueSetting{}, spec.Valid); len(parts) > 0 {
		value = parts[0]
	}
	if value == "" {
		value = spec.Fallback
	}
	if parts := read(t, spec.KeysCredit, MultiValueSetting{}, nil); len(parts) > 0 && parts[0] != value {
		valueCredit = parts[0]
	}
	return
}

func read(t Tags, c Keys, setting MultiValueSetting, valid func(string) bool) []string {
	if valid == nil {
		valid = func(v string) bool { return v != "" }
	}
	accept := func(parts []string) bool {
		return slices.ContainsFunc(parts, valid)
	}

	var parts []string
	switch setting.Mode {
	case Multi:
		for _, k := range c.MultiKey {
			if v := normtag.Values(t, k); accept(v) {
				parts = slices.Clone(v)
				break
			}
		}
		if parts == nil {
			for _, k := range c.Key {
				if v := normtag.Values(t, k); accept(v) {
					parts = slices.Clone(v)
					break
				}
			}
		}
	case Delim:
		for _, k := range c.Key {
			if v := normtag.Get(t, k); v != "" {
				if p := strings.Split(v, setting.Delim); accept(p) {
					parts = p
					break
				}
			}
		}
	default:
		for _, k := range c.Key {
			if v := normtag.Get(t, k); accept([]string{v}) {
				parts = []string{v}
				break
			}
		}
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

type Credited struct {
	Value, ValueCredit string
}

func PairCredits(values, valuesCredit []string) []Credited {
	out := make([]Credited, 0, len(values))
	for i, v := range values {
		if v == "" {
			continue
		}
		e := Credited{Value: v}
		if i < len(valuesCredit) && valuesCredit[i] != "" && valuesCredit[i] != v {
			e.ValueCredit = valuesCredit[i]
		}
		out = append(out, e)
	}
	return out
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
