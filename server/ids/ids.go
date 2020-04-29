package ids

// this package is at such a high level in the hierarchy because
// it's used by both `server/db` (for now) and `server/ctrlsubsonic`

import (
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

type ID string

const (
	// type values copied from subsonic
	Artist ID = "ar"
	Album  ID = "al"
	Track  ID = "tr"
)

var accepted = []ID{Artist,
	Album,
	Track,
}

type IDV struct {
	Type  ID
	Value int
}

func (i IDV) String() string {
	return fmt.Sprintf("%s-%d", i.Type, i.Value)
}

func Parse(in string) (IDV, error) {
	parts := strings.Split(in, "-")
	if len(parts) != 2 {
		return IDV{}, ErrBadSeparator
	}
	partType := parts[0]
	partValue := parts[1]
	val, err := strconv.Atoi(partValue)
	if err != nil {
		return IDV{}, fmt.Errorf("%q: %w", partValue, ErrNotAnInt)
	}
	for _, acc := range accepted {
		if partType == string(acc) {
			return IDV{Type: acc, Value: val}, nil
		}
	}
	return IDV{}, fmt.Errorf("%q: %w", partType, ErrBadPrefix)
}
