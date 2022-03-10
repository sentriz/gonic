package httprange

import (
	"fmt"
	"regexp"
	"strconv"
)

type Unit string

const (
	UnitBytes Unit = "bytes"
)

//nolint:gochecknoglobals
var (
	reg   = regexp.MustCompile(`^(?P<unit>\w+)=(?P<start>(?:\d+)?)\s*-\s*(?P<end>(?:\d+)?)$`)
	unit  = reg.SubexpIndex("unit")
	start = reg.SubexpIndex("start")
	end   = reg.SubexpIndex("end")
)

var (
	ErrInvalidRange = fmt.Errorf("invalid range")
	ErrUnknownUnit  = fmt.Errorf("unknown range")
)

type Range struct {
	Start, End, Length int // bytes
	Partial            bool
}

func Parse(in string, fullLength int) (Range, error) {
	parts := reg.FindStringSubmatch(in)
	if len(parts)-1 != reg.NumSubexp() {
		return Range{0, fullLength - 1, fullLength, false}, nil
	}

	switch unit := parts[unit]; Unit(unit) {
	case UnitBytes:
	default:
		return Range{}, fmt.Errorf("%q: %w", unit, ErrUnknownUnit)
	}

	start, _ := strconv.Atoi(parts[start])
	end, _ := strconv.Atoi(parts[end])
	length := fullLength
	partial := false

	switch {
	case end > 0 && end < length:
		length = end - start + 1
		partial = true
	case end == 0 && length > 0:
		end = length - 1
	}

	return Range{start, end, length, partial}, nil
}
