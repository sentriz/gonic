package fileutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUniquePath(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

func TestSafe(t *testing.T) {
	t.Parallel()

	longName := "hi_00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
	result := Safe(longName)
	require.Equal(t, 200, len(result))
	require.Equal(t, longName[:200], result)

	longNameWithExt := "hi_00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000.mp3"
	result = Safe(longNameWithExt)
	require.Equal(t, 200, len(result))
	require.Equal(t, ".mp3", result[len(result)-4:])

	require.Equal(t, "test.mp3", Safe("test.mp3"))
	require.Equal(t, "testing.mp3", Safe("test ing.mp3"))
}
