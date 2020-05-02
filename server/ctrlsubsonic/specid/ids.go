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
	Artist    IDT = "ar"
	Album     IDT = "al"
	Track     IDT = "tr"
	separator     = "-"
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
	for _, acc := range []IDT{Artist, Album, Track} {
		if partType == string(acc) {
			return ID{Type: acc, Value: val}, nil
		}
	}
	return ID{}, fmt.Errorf("%q: %w", partType, ErrBadPrefix)
}

func (i ID) String() string {
	return fmt.Sprintf("%s%s%d", i.Type, separator, i.Value)
}

func (i ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}
