package specid

// this package is at such a high level in the hierarchy because
// it's used by both `server/db` (for now) and `server/ctrlsubsonic`

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrBadSeparator = errors.New("bad separator")
	ErrNotAnInt     = errors.New("not an int")
	ErrBadPrefix    = errors.New("bad prefix")
	ErrBadJSON      = errors.New("bad JSON")
)

type IDT string

const (
	Artist               IDT = "ar"
	Album                IDT = "al"
	Track                IDT = "tr"
	Podcast              IDT = "pd"
	PodcastEpisode       IDT = "pe"
	InternetRadioStation IDT = "ir"
	Playlist             IDT = "pl"
	separator                = "-"
)

//nolint:musttag
type ID struct {
	Type        IDT
	Value       int
	StringValue string
}

func New(in string) (ID, error) {
	partType, partValue, ok := strings.Cut(in, separator)
	if !ok {
		return ID{}, ErrBadSeparator
	}

	switch IDT(partType) {
	case Playlist:
		return ID{Type: Playlist, StringValue: partValue}, nil
	}

	val, err := strconv.Atoi(partValue)
	if err != nil {
		return ID{}, fmt.Errorf("%q: %w", partValue, ErrNotAnInt)
	}

	switch IDT(partType) {
	case Artist:
		return ID{Type: Artist, Value: val}, nil
	case Album:
		return ID{Type: Album, Value: val}, nil
	case Track:
		return ID{Type: Track, Value: val}, nil
	case Podcast:
		return ID{Type: Podcast, Value: val}, nil
	case PodcastEpisode:
		return ID{Type: PodcastEpisode, Value: val}, nil
	case InternetRadioStation:
		return ID{Type: InternetRadioStation, Value: val}, nil
	default:
		return ID{}, fmt.Errorf("%q: %w", partType, ErrBadPrefix)
	}
}

func (i ID) String() string {
	switch {
	case i.Value != 0:
		return fmt.Sprintf("%s%s%d", i.Type, separator, i.Value)
	case i.StringValue != "":
		return fmt.Sprintf("%s%s%s", i.Type, separator, i.StringValue)
	}
	return "-1"
}

func (i ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

func (i *ID) UnmarshalJSON(data []byte) error {
	if len(data) <= 2 {
		return fmt.Errorf("too short: %w", ErrBadJSON)
	}
	id, err := New(string(data[1 : len(data)-1])) // Strip quotes
	if err == nil {
		*i = id
	}
	return err
}

func (i ID) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}
