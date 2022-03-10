package httprange_test

import (
	"testing"

	"github.com/matryer/is"
	"go.senan.xyz/gonic/server/ctrlsubsonic/httprange"
)

func TestParse(t *testing.T) {
	is := is.New(t)

	full := func(start, end, length int) httprange.Range {
		return httprange.Range{Start: start, End: end, Length: length}
	}
	partial := func(start, end, length int) httprange.Range {
		return httprange.Range{Start: start, End: end, Length: length, Partial: true}
	}
	parse := func(in string, length int) httprange.Range {
		is.Helper()
		rrange, err := httprange.Parse(in, length)
		is.NoErr(err)
		return rrange
	}

	is.Equal(parse("bytes=0-0", 0), full(0, 0, 0))
	is.Equal(parse("bytes=0-", 10), full(0, 9, 10))
	is.Equal(parse("bytes=0-49", 50), partial(0, 49, 50))
	is.Equal(parse("bytes=50-99", 100), partial(50, 99, 50))
}
