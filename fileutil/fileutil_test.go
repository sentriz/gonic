package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUniquePath(t *testing.T) {
	unq := func(base, filename string, count uint) string {
		r, err := unique(base, filename, count)
		require.NoError(t, err)
		return r
	}

	require.Equal(t, "test/wow.mp3", unq("test", "wow.mp3", 0))
	require.Equal(t, "test/wow (1).mp3", unq("test", "wow.mp3", 1))
	require.Equal(t, "test/wow (2).mp3", unq("test", "wow.mp3", 2))

	require.Equal(t, "test", unq("test", "", 0))
	require.Equal(t, "test (1)", unq("test", "", 1))

	base := filepath.Join(t.TempDir(), "a")

	require.NoError(t, os.MkdirAll(base, os.ModePerm))

	next := base + " (1)"
	require.Equal(t, next, unq(base, "", 0))

	require.NoError(t, os.MkdirAll(next, os.ModePerm))

	next = base + " (2)"
	require.Equal(t, next, unq(base, "", 0))

	_, err := os.Create(filepath.Join(base, "test.mp3"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(base, "test (1).mp3"), unq(base, "test.mp3", 0))
}

func TestFirst(t *testing.T) {
	base := t.TempDir()
	name := filepath.Join(base, "test")
	_, err := os.Create(name)
	require.NoError(t, err)

	p := func(name string) string {
		return filepath.Join(base, name)
	}

	r, err := First(p("one"), p("two"), p("test"), p("four"))
	require.NoError(t, err)
	require.Equal(t, p("test"), r)

}
