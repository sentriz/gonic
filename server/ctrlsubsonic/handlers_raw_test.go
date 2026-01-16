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

	// Test case 1: Cover found with .jpg extension
	t.Run("cover found jpg", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		// Create covers directory inside user folder
		coversDir := filepath.Join(tmp, "1", "covers")
		require.NoError(t, os.MkdirAll(coversDir, 0o755))

		playlistPath := "1/test-playlist.m3u"
		coverPath := filepath.Join(coversDir, "test-playlist.jpg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		file, err := coverForPlaylist(store, playlistPath)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	// Test case 2: Cover found with .jpeg extension
	t.Run("cover found jpeg", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		// Create covers directory inside user folder
		coversDir := filepath.Join(tmp, "1", "covers")
		require.NoError(t, os.MkdirAll(coversDir, 0o755))

		playlistPath := "1/test-playlist.m3u"
		coverPath := filepath.Join(coversDir, "test-playlist.jpeg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		file, err := coverForPlaylist(store, playlistPath)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	// Test case 3: Cover not found
	t.Run("cover not found", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		// Create covers directory inside user folder but don't create any cover files
		coversDir := filepath.Join(tmp, "1", "covers")
		require.NoError(t, os.MkdirAll(coversDir, 0o755))

		playlistPath := "1/nonexistent-playlist.m3u"
		file, err := coverForPlaylist(store, playlistPath)
		require.Error(t, err)
		require.Nil(t, file)
		require.Equal(t, errCoverEmpty, err)
	})

	// Test case 4: Playlist name extraction from nested path
	t.Run("nested playlist path", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		// Create covers directory inside user folder
		coversDir := filepath.Join(tmp, "1", "covers")
		require.NoError(t, os.MkdirAll(coversDir, 0o755))

		nestedPath := "1/subfolder/my-nested-playlist.m3u"
		coverPath := filepath.Join(coversDir, "my-nested-playlist.jpg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		file, err := coverForPlaylist(store, nestedPath)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	// Test case 5: Playlist with .m3u8 extension
	t.Run("playlist m3u8 extension", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		// Create covers directory inside user folder
		coversDir := filepath.Join(tmp, "1", "covers")
		require.NoError(t, os.MkdirAll(coversDir, 0o755))

		m3u8Path := "1/test-playlist.m3u8"
		coverPath := filepath.Join(coversDir, "test-playlist.jpg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		file, err := coverForPlaylist(store, m3u8Path)
		require.NoError(t, err)
		require.NotNil(t, file)
		require.NoError(t, file.Close())
	})

	// Test case 6: Covers directory doesn't exist
	t.Run("covers directory missing", func(t *testing.T) {
		t.Parallel()

		tmp := t.TempDir()
		store, err := playlist.NewStore(tmp)
		require.NoError(t, err)

		// Don't create covers directory
		playlistPath := "1/test-playlist.m3u"
		file, err := coverForPlaylist(store, playlistPath)
		require.Error(t, err)
		require.Nil(t, file)
		require.Equal(t, errCoverEmpty, err)
	})
}

func TestDecodePlaylistID(t *testing.T) {
	t.Parallel()

	tcases := []struct {
		name     string
		id       string
		expected string
	}{
		{
			name:     "valid base64 playlist id",
			id:       "MS9teS1wbGF5bGlzdC5tM3U=", // base64 of "1/my-playlist.m3u"
			expected: "1/my-playlist.m3u",
		},
		{
			name:     "empty string",
			id:       "",
			expected: "",
		},
		{
			name:     "nested path",
			id:       "MS9zdWJmb2xkZXIvbXktbmVzdGVkLXBsYXlsaXN0Lm0zdQ==", // base64 of "1/subfolder/my-nested-playlist.m3u"
			expected: "1/subfolder/my-nested-playlist.m3u",
		},
	}

	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			result := decodePlaylistID(tcase.id)
			require.Equal(t, tcase.expected, result)
		})
	}
}
