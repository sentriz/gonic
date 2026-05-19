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
		MultiKey: []string{normtag.Artists}, Key: []string{normtag.Artist},
		MultiKeyCredit: []string{normtag.ArtistsCredit}, KeyCredit: []string{normtag.ArtistCredit},
		Fallback: "Unknown Artist",
	}
	AlbumArtist = &Spec{
		MultiKey: []string{normtag.AlbumArtists}, Key: []string{normtag.AlbumArtist, normtag.Artist},
		MultiKeyCredit: []string{normtag.AlbumArtistsCredit}, KeyCredit: []string{normtag.AlbumArtistCredit},
		Fallback: "Unknown Artist",
	}
	Genre = &Spec{
		MultiKey: []string{normtag.Genres}, Key: []string{normtag.Genre},
		Fallback: "Unknown Genre",
	}
	ISRC = &Spec{
		MultiKey: []string{normtag.ISRC}, Key: []string{normtag.ISRC},
	}

	Remixer = &Spec{
		MultiKey: []string{normtag.Remixers}, Key: []string{normtag.Remixer},
		MultiKeyCredit: []string{normtag.RemixersCredit}, KeyCredit: []string{normtag.RemixerCredit},
	}
	Composer = &Spec{
		MultiKey: []string{normtag.Composers}, Key: []string{normtag.Composer},
		MultiKeyCredit: []string{normtag.ComposersCredit}, KeyCredit: []string{normtag.ComposerCredit},
	}
	Lyricist = &Spec{
		MultiKey: []string{normtag.Lyricists}, Key: []string{normtag.Lyricist},
		MultiKeyCredit: []string{normtag.LyricistsCredit}, KeyCredit: []string{normtag.LyricistCredit},
	}
	Conductor = &Spec{
		MultiKey: []string{normtag.Conductors}, Key: []string{normtag.Conductor},
		MultiKeyCredit: []string{normtag.ConductorsCredit}, KeyCredit: []string{normtag.ConductorCredit},
	}
	Producer = &Spec{
		MultiKey: []string{normtag.Producers}, Key: []string{normtag.Producer},
		MultiKeyCredit: []string{normtag.ProducersCredit}, KeyCredit: []string{normtag.ProducerCredit},
	}
	Arranger = &Spec{
		MultiKey: []string{normtag.Arrangers}, Key: []string{normtag.Arranger},
		MultiKeyCredit: []string{normtag.ArrangersCredit}, KeyCredit: []string{normtag.ArrangerCredit},
	}

	AlbumTitle = &Spec{
		Key:      []string{normtag.Album},
		Fallback: "Unknown Album",
	}
	TrackTitle = &Spec{
		Key: []string{normtag.Title},
	}
	Year = &Spec{
		Key:   []string{normtag.OriginalDate, normtag.Date},
		Valid: func(v string) bool { return !ParseDate(v).IsZero() },
	}
)

type Spec struct {
	MultiKey, Key             []string
	MultiKeyCredit, KeyCredit []string
	Fallback                  string
	Valid                     func(string) bool
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

	values = read(t, spec.MultiKey, spec.Key, setting, spec.Valid)
	if len(values) == 0 && spec.Fallback != "" {
		values = []string{spec.Fallback}
	}

	valuesCredit = read(t, spec.MultiKeyCredit, spec.KeyCredit, setting, nil)

	return values, valuesCredit
}

func Read(t Tags, spec *Spec) (value, valueCredit string) {
	if parts := read(t, spec.MultiKey, spec.Key, MultiValueSetting{}, spec.Valid); len(parts) > 0 {
		value = parts[0]
	}
	if value == "" {
		value = spec.Fallback
	}
	if parts := read(t, spec.MultiKeyCredit, spec.KeyCredit, MultiValueSetting{}, nil); len(parts) > 0 && parts[0] != value {
		valueCredit = parts[0]
	}
	return
}

func read(t Tags, multiKey, key []string, setting MultiValueSetting, valid func(string) bool) []string {
	if valid == nil {
		valid = func(v string) bool { return v != "" }
	}
	accept := func(parts []string) bool {
		return slices.ContainsFunc(parts, valid)
	}

	var parts []string
	switch setting.Mode {
	case Multi:
		for _, k := range multiKey {
			if v := normtag.Values(t, k); accept(v) {
				parts = slices.Clone(v)
				break
			}
		}
		if parts == nil {
			for _, k := range key {
				if v := normtag.Values(t, k); accept(v) {
					parts = slices.Clone(v)
					break
				}
			}
		}
	case Delim:
		for _, k := range key {
			if v := normtag.Get(t, k); v != "" {
				if p := strings.Split(v, setting.Delim); accept(p) {
					parts = p
					break
				}
			}
		}
	default:
		for _, k := range key {
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
