package texttree_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/texttree"
)

func TestParseReader(t *testing.T) {
	t.Parallel()

	tree, err := texttree.ParseReader(strings.NewReader("electronic\tedm\nedm\ttechno\n")) //nolint:dupword
	require.NoError(t, err)

	check := func(name string, want []string) {
		t.Helper()
		got := append([]string{}, tree[name]...)
		sort.Strings(got)
		sort.Strings(want)
		assert.Equal(t, want, got, name)
	}

	check("electronic", []string{"edm", "electronic", "techno"})
	check("edm", []string{"edm", "techno"})
	check("techno", []string{"techno"})
}

func TestParseReaderCycle(t *testing.T) {
	t.Parallel()

	_, err := texttree.ParseReader(strings.NewReader("a\tb\nb\tc\nc\ta\n")) //nolint:dupword
	assert.Error(t, err)
}

func TestParseReaderDuplicateChild(t *testing.T) {
	t.Parallel()

	_, err := texttree.ParseReader(strings.NewReader("a\tb\nc\tb\n"))
	assert.Error(t, err)
}
