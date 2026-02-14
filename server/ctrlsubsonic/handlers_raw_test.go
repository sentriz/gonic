package ctrlsubsonic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/playlist"
)

func TestCoverForPlaylist(t *testing.T) {
	t.Parallel()

	t.Run("cover found jpg", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "test-playlist.jpg")))

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	t.Run("cover found png", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "test-playlist.png")))

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	t.Run("cover not found", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.ErrorIs(t, err, errCoverEmpty)
		require.Nil(t, file)
	})

	t.Run("nested playlist path", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1", "subfolder")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "my-nested-playlist.jpg")))

		playlistID := playlistIDEncode("1/subfolder/my-nested-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	t.Run("playlist directory missing", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistID := playlistIDEncode("1/test-playlist.m3u")
		file, err := coverForPlaylist(store, playlistID)
		require.Error(t, err)
		require.Nil(t, file)
	})

	t.Run("different playlists different covers", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		playlistDir := filepath.Join(tmp, "1")
		require.NoError(t, os.MkdirAll(playlistDir, 0o755))
		require.NoError(t, touch(filepath.Join(playlistDir, "playlist-a.jpg")))
		require.NoError(t, touch(filepath.Join(playlistDir, "playlist-b.png")))

		fileA, err := coverForPlaylist(store, playlistIDEncode("1/playlist-a.m3u"))
		require.NoError(t, err)
		require.Equal(t, "playlist-a.jpg", filepath.Base(fileA.Name()))
		require.NoError(t, fileA.Close())

		fileB, err := coverForPlaylist(store, playlistIDEncode("1/playlist-b.m3u"))
		require.NoError(t, err)
		require.Equal(t, "playlist-b.png", filepath.Base(fileB.Name()))
		require.NoError(t, fileB.Close())
	})
}

func touch(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	return f.Close()
}
