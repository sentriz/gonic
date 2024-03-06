package mpd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArgParser(t *testing.T) {
	t.Parallel()

	expectations := map[string][]string{
		`cmd`:                                  {"cmd"},
		`cmd arg`:                              {"cmd", "arg"},
		`cmd "arg one" arg-two`:                {"cmd", "arg one", "arg-two"},
		`find "(Artist == \"foo\\'bar\\\"\")"`: {"find", `(Artist == "foo\'bar\"")`},
	}

	for line, args := range expectations {
		p := newArgParser(line)

		for i, expected := range args {
			j, parsed, ok := p.Next()
			assert.True(t, ok)
			assert.Equal(t, uint(i+1), j)
			assert.Equal(t, expected, parsed)
		}

		_, arg, ok := p.Next()
		assert.False(t, ok, "parser return extra arg: %q", arg)
	}
}
