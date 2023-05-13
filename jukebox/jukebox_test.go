package jukebox_test

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert := assert.New(t)

	assert.NoError(j.SetPlaylist([]string{
		testPath("tr_0.mp3"),
		testPath("tr_1.mp3"),
		testPath("tr_2.mp3"),
		testPath("tr_3.mp3"),
		testPath("tr_4.mp3"),
	}))

	status, err := j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 0)
	assert.Equal(status.CurrentFilename, testPath("tr_0.mp3"))
	assert.Equal(status.Length, 5)
	assert.Equal(status.Playing, true)

	items, err := j.GetPlaylist()
	assert.NoError(err)

	itemsSorted := append([]string(nil), items...)
	sort.Strings(itemsSorted)
	assert.Equal(items, itemsSorted)

	assert.NoError(j.Play())

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.Playing, true)

	assert.NoError(j.Pause())

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.Playing, false)

	assert.NoError(j.Play())

	// skip to 2
	assert.NoError(j.SkipToPlaylistIndex(2, 0))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 2)
	assert.Equal(status.CurrentFilename, testPath("tr_2.mp3"))
	assert.Equal(status.Length, 5)
	assert.Equal(status.Playing, true)

	// skip to 3
	assert.NoError(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 3)
	assert.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	assert.Equal(status.Length, 5)
	assert.Equal(status.Playing, true)

	// just add one more by overwriting the playlist like some clients do
	// we should keep the current track unchaned if we find it
	assert.NoError(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
		"testdata/tr_4.mp3",
		"testdata/tr_5.mp3",
	}))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 3) // index unchanged
	assert.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	assert.Equal(status.Length, 6) // we added one more track
	assert.Equal(status.Playing, true)

	// skip to 3 again
	assert.NoError(j.SkipToPlaylistIndex(3, 0))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 3)
	assert.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	assert.Equal(status.Length, 6)
	assert.Equal(status.Playing, true)

	// remove all but 3
	assert.NoError(j.SetPlaylist([]string{
		"testdata/tr_0.mp3",
		"testdata/tr_1.mp3",
		"testdata/tr_2.mp3",
		"testdata/tr_3.mp3",
	}))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 3) // index unchanged
	assert.Equal(status.CurrentFilename, testPath("tr_3.mp3"))
	assert.Equal(status.Length, 4)
	assert.Equal(status.Playing, true)

	// skip to 2 (5s long) in the middle of the track
	assert.NoError(j.SkipToPlaylistIndex(2, 2))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 2) // index unchanged
	assert.Equal(status.CurrentFilename, testPath("tr_2.mp3"))
	assert.Equal(status.Length, 4)
	assert.Equal(status.Playing, true)
	assert.Equal(status.Position, 2) // at new position

	// overwrite completely
	assert.NoError(j.SetPlaylist([]string{
		"testdata/tr_5.mp3",
		"testdata/tr_6.mp3",
		"testdata/tr_7.mp3",
		"testdata/tr_8.mp3",
		"testdata/tr_9.mp3",
	}))

	status, err = j.GetStatus()
	assert.NoError(err)
	assert.Equal(status.CurrentIndex, 0) // index unchanged
	assert.Equal(status.CurrentFilename, testPath("tr_5.mp3"))
	assert.Equal(status.Length, 5)
	assert.Equal(status.Playing, true)
}

func TestVolume(t *testing.T) {
	t.Parallel()
	j := newJukebox(t)
	assert := assert.New(t)

	vol, err := j.GetVolumePct()
	assert.NoError(err)
	assert.Equal(vol, 100.0)

	assert.NoError(j.SetVolumePct(69.0))

	vol, err = j.GetVolumePct()
	assert.NoError(err)
	assert.Equal(vol, 69.0)

	assert.NoError(j.SetVolumePct(0.0))

	vol, err = j.GetVolumePct()
	assert.NoError(err)
	assert.Equal(vol, 0.0)
}

func testPath(path string) string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "testdata", path)
}
