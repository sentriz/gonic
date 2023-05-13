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

func newJukebox(t *testing.T) *jukebox.Jukebox {
	sockPath := filepath.Join(t.TempDir(), "mpv.sock")

	j := jukebox.New()
	err := j.Start(
		sockPath,
		[]string{jukebox.MPVArg("--ao", "null")},
	)
	if errors.Is(err, jukebox.ErrMPVTooOld) {
		t.Skip("old mpv found, skipping")
	}
	if err != nil {
		t.Fatalf("start jukebox: %v", err)
	}
	t.Cleanup(func() {
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
	require.Equal(status.CurrentIndex, 0)
	require.Equal(status.CurrentFilename, testPath("tr_0.mp3"))
	require.Equal(status.Length, 5)
	require.Equal(status.Playing, true)

	items, err := j.GetPlaylist()
	require.NoError(err)

	itemsSorted := append([]string(nil), items...)
	sort.Strings(itemsSorted)
	require.Equal(items, itemsSorted)

	require.NoError(j.Play())

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.Playing, true)

	require.NoError(j.Pause())

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.Playing, false)

	require.NoError(j.Play())

	// skip to 2
	require.NoError(j.SkipToPlaylistIndex(2, 0))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.CurrentIndex, 2)
	require.Equal(status.CurrentFilename, testPath("tr_2.mp3"))
	require.Equal(status.Length, 5)
	require.Equal(status.Playing, true)

	// skip to 3
	require.NoError(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.CurrentIndex, 3)
	require.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	require.Equal(status.Length, 5)
	require.Equal(status.Playing, true)

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
	require.Equal(status.CurrentIndex, 3) // index unchanged
	require.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	require.Equal(status.Length, 6) // we added one more track
	require.Equal(status.Playing, true)

	// skip to 3 again
	require.NoError(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.CurrentIndex, 3)
	require.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	require.Equal(status.Length, 6)
	require.Equal(status.Playing, true)

	// remove all but 3
	require.NoError(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
	}))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.CurrentIndex, 3) // index unchanged
	require.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	require.Equal(status.Length, 4)
	require.Equal(status.Playing, true)

	// skip to 2 (5s long) in the middle of the track
	require.NoError(j.SkipToPlaylistIndex(2, 2))

	status, err = j.GetStatus()
	require.NoError(err)
	require.Equal(status.CurrentIndex, 2) // index unchanged
	require.Equal(status.CurrentFilename, testPath("tr_2.mp3"))
	require.Equal(status.Length, 4)
	require.Equal(status.Playing, true)
	require.Equal(status.Position, 2) // at new position

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
	require.Equal(status.CurrentIndex, 0) // index unchanged
	require.Equal(status.CurrentFilename, testPath("tr_5.mp3"))
	require.Equal(status.Length, 5)
	require.Equal(status.Playing, true)
}

func TestVolume(t *testing.T) {
	t.Parallel()
	j := newJukebox(t)
	require := require.New(t)

	vol, err := j.GetVolumePct()
	require.NoError(err)
	require.Equal(vol, 100.0)

	require.NoError(j.SetVolumePct(69.0))

	vol, err = j.GetVolumePct()
	require.NoError(err)
	require.Equal(vol, 69.0)

	require.NoError(j.SetVolumePct(0.0))

	vol, err = j.GetVolumePct()
	require.NoError(err)
	require.Equal(vol, 0.0)
}

func testPath(path string) string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "testdata", path)
}
