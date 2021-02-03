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
)

type IDT string

const (
	Artist         IDT = "ar"
	Album          IDT = "al"
	Track          IDT = "tr"
	Podcast        IDT = "pd"
	PodcastEpisode IDT = "pe"
	separator          = "-"
)

type ID struct {
	Type  IDT
	Value int
}

func New(in string) (ID, error) {
	parts := strings.Split(in, separator)
	if len(parts) != 2 {
		return ID{}, ErrBadSeparator
	}
	partType := parts[0]
	partValue := parts[1]
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
	default:
		return ID{}, fmt.Errorf("%q: %w", partType, ErrBadPrefix)
	}
}

func (i ID) String() string {
	if i.Value == 0 {
		return "-1"
	}
	return fmt.Sprintf("%s%s%d", i.Type, separator, i.Value)
}

func (i ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

func (i ID) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}
