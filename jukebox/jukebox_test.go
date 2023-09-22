package jukebox_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/jukebox"
)

func newJukebox(tb testing.TB) *jukebox.Jukebox {
	tb.Helper()

	sockPath := filepath.Join(tb.TempDir(), "mpv.sock")

	j := jukebox.New()
	err := j.Start(
		sockPath,
		[]string{jukebox.MPVArg("--ao", "null")},
	)
	if errors.Is(err, jukebox.ErrMPVTooOld) {
		tb.Skip("old mpv found, skipping")
	}
	if err != nil {
		tb.Fatalf("start jukebox: %v", err)
	}
	tb.Cleanup(func() {
		j.Quit()
	})
	return j
}

func TestPlaySkipReset(t *testing.T) {
	t.Skip("bit flakey currently")

	t.Parallel()
	j := newJukebox(t)
	require := require.New(t)

	require.NoError(j.SetPlaylist([]string{
		testPath("tr_0.mp3"),
		testPath("tr_1.mp3"),
		testPath("tr_2.mp3"),
		testPath("tr_3.mp3"),
		testPath("tr_4.mp3"),
	}))

	status, err := j.GetStatus()
	require.NoError(err)
	require.Equal(0, status.CurrentIndex)
	require.Equal(testPath("tr_0.mp3"), status.CurrentFilename)
	require.Equal(5, status.Length)
	require.Equal(true, status.Playing)

	items, err := j.GetPlaylist()
	require.NoError(err)

	itemsSorted := append([]string(nil), items...)
	sort.Strings(itemsSorted)
	require.Equal(itemsSorted, items)

	require.NoError(j.Play())

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(true, status.Playing)

	require.NoError(j.Pause())

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(false, status.Playing)

	require.NoError(j.Play())

	// skip to 2
	require.NoError(j.SkipToPlaylistIndex(2, 0))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(2, status.CurrentIndex)
	require.Equal(testPath("tr_2.mp3"), status.CurrentFilename)
	require.Equal(5, status.Length)
	require.Equal(true, status.Playing)

	// skip to 3
	require.NoError(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(3, status.CurrentIndex)
	require.Equal(testPath("tr_3.mp3"), status.CurrentFilename)
	require.Equal(5, status.Length)
	require.Equal(true, status.Playing)

	// just add one more by overwriting the playlist like some clients do
	// we should keep the current track unchaned if we find it
	require.NoError(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
		"testdata/tr_4.mp3",
		"testdata/tr_5.mp3",
	}))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(3, status.CurrentIndex) // index unchanged
	require.Equal(testPath("tr_3.mp3"), status.CurrentFilename)
	require.Equal(6, status.Length) // we added one more track
	require.Equal(true, status.Playing)

	// skip to 3 again
	require.NoError(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(3, status.CurrentIndex)
	require.Equal(testPath("tr_3.mp3"), status.CurrentFilename)
	require.Equal(6, status.Length)
	require.Equal(true, status.Playing)

	// remove all but 3
	require.NoError(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
	}))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(3, status.CurrentIndex) // index unchanged
	require.Equal(testPath("tr_3.mp3"), status.CurrentFilename)
	require.Equal(4, status.Length)
	require.Equal(true, status.Playing)

	// skip to 2 (5s long) in the middle of the track
	require.NoError(j.SkipToPlaylistIndex(2, 2))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(2, status.CurrentIndex) // index unchanged
	require.Equal(testPath("tr_2.mp3"), status.CurrentFilename)
	require.Equal(4, status.Length)
	require.Equal(true, status.Playing)
	require.Equal(2, status.Position) // at new position

	// overwrite completely
	require.NoError(j.SetPlaylist([]string{
		"testdata/tr_5.mp3",
		"testdata/tr_6.mp3",
		"testdata/tr_7.mp3",
		"testdata/tr_8.mp3",
		"testdata/tr_9.mp3",
	}))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(0, status.CurrentIndex) // index unchanged
	require.Equal(testPath("tr_5.mp3"), status.CurrentFilename)
	require.Equal(5, status.Length)
	require.Equal(true, status.Playing)
}

func TestVolume(t *testing.T) {
	t.Parallel()
	j := newJukebox(t)
	require := require.New(t)

	vol, err := j.GetVolumePct()
	require.NoError(err)
	require.Equal(100.0, vol)

	require.NoError(j.SetVolumePct(69.0))

	vol, err = j.GetVolumePct()
	require.NoError(err)
	require.Equal(69.0, vol)

	require.NoError(j.SetVolumePct(0.0))

	vol, err = j.GetVolumePct()
	require.NoError(err)
	require.Equal(0.0, vol)
}

func testPath(path string) string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "testdata", path)
}
