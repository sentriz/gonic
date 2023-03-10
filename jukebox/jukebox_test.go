package jukebox_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/matryer/is"
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
	is := is.New(t)

	is.NoErr(j.SetPlaylist([]string{
		testPath("tr_0.mp3"),
		testPath("tr_1.mp3"),
		testPath("tr_2.mp3"),
		testPath("tr_3.mp3"),
		testPath("tr_4.mp3"),
	}))

	status, err := j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 0)
	is.Equal(status.CurrentFilename, testPath("tr_0.mp3"))
	is.Equal(status.Length, 5)
	is.Equal(status.Playing, true)

	items, err := j.GetPlaylist()
	is.NoErr(err)

	itemsSorted := append([]string(nil), items...)
	sort.Strings(itemsSorted)
	is.Equal(items, itemsSorted)

	is.NoErr(j.Play())

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.Playing, true)

	is.NoErr(j.Pause())

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.Playing, false)

	is.NoErr(j.Play())

	// skip to 2
	is.NoErr(j.SkipToPlaylistIndex(2, 0))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 2)
	is.Equal(status.CurrentFilename, testPath("tr_2.mp3"))
	is.Equal(status.Length, 5)
	is.Equal(status.Playing, true)

	// skip to 3
	is.NoErr(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 3)
	is.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	is.Equal(status.Length, 5)
	is.Equal(status.Playing, true)

	// just add one more by overwriting the playlist like some clients do
	// we should keep the current track unchaned if we find it
	is.NoErr(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
		"testdata/tr_4.mp3",
		"testdata/tr_5.mp3",
	}))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 3) // index unchanged
	is.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	is.Equal(status.Length, 6) // we added one more track
	is.Equal(status.Playing, true)

	// skip to 3 again
	is.NoErr(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 3)
	is.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	is.Equal(status.Length, 6)
	is.Equal(status.Playing, true)

	// remove all but 3
	is.NoErr(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
	}))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 3) // index unchanged
	is.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	is.Equal(status.Length, 4)
	is.Equal(status.Playing, true)

	// skip to 2 (5s long) in the middle of the track
	is.NoErr(j.SkipToPlaylistIndex(2, 2))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 2) // index unchanged
	is.Equal(status.CurrentFilename, testPath("tr_2.mp3"))
	is.Equal(status.Length, 4)
	is.Equal(status.Playing, true)
	is.Equal(status.Position, 2) // at new position

	// overwrite completely
	is.NoErr(j.SetPlaylist([]string{
		"testdata/tr_5.mp3",
		"testdata/tr_6.mp3",
		"testdata/tr_7.mp3",
		"testdata/tr_8.mp3",
		"testdata/tr_9.mp3",
	}))

	status, err = j.GetStatus()
	is.NoErr(err)
	is.Equal(status.CurrentIndex, 0) // index unchanged
	is.Equal(status.CurrentFilename, testPath("tr_5.mp3"))
	is.Equal(status.Length, 5)
	is.Equal(status.Playing, true)
}

func TestVolume(t *testing.T) {
	t.Parallel()
	j := newJukebox(t)
	is := is.New(t)

	vol, err := j.GetVolumePct()
	is.NoErr(err)
	is.Equal(vol, 100.0)

	is.NoErr(j.SetVolumePct(69.0))

	vol, err = j.GetVolumePct()
	is.NoErr(err)
	is.Equal(vol, 69.0)

	is.NoErr(j.SetVolumePct(0.0))

	vol, err = j.GetVolumePct()
	is.NoErr(err)
	is.Equal(vol, 0.0)
}

func testPath(path string) string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "testdata", path)
}
