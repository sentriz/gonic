package ctrlsubsonic

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

const testPlaylistPath = "1/test-playlist.m3u"

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

		playlistPath := testPlaylistPath
		coverPath := filepath.Join(coversDir, "test-playlist.jpg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		idMap := newPlaylistIDMap()
		playlistID := idMap.pathToSpecID(playlistPath)
		file, err := coverForPlaylist(store, idMap, playlistID)
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

		playlistPath := testPlaylistPath
		coverPath := filepath.Join(coversDir, "test-playlist.jpeg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		idMap := newPlaylistIDMap()
		playlistID := idMap.pathToSpecID(playlistPath)
		file, err := coverForPlaylist(store, idMap, playlistID)
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
		idMap := newPlaylistIDMap()
		playlistID := idMap.pathToSpecID(playlistPath)
		file, err := coverForPlaylist(store, idMap, playlistID)
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

		idMap := newPlaylistIDMap()
		playlistID := idMap.pathToSpecID(nestedPath)
		file, err := coverForPlaylist(store, idMap, playlistID)
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

		m3u8Path := "1/test-playlist.m3u8" // Different path for m3u8 test
		coverPath := filepath.Join(coversDir, "test-playlist.jpg")
		f, err := os.Create(coverPath)
		require.NoError(t, err)
		require.NoError(t, f.Close())

		idMap := newPlaylistIDMap()
		playlistID := idMap.pathToSpecID(m3u8Path)
		file, err := coverForPlaylist(store, idMap, playlistID)
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
		playlistPath := testPlaylistPath
		idMap := newPlaylistIDMap()
		playlistID := idMap.pathToSpecID(playlistPath)
		file, err := coverForPlaylist(store, idMap, playlistID)
		require.Error(t, err)
		require.Nil(t, file)
		require.Equal(t, errCoverEmpty, err)
	})
}

func TestPlaylistIDMap(t *testing.T) {
	t.Parallel()

	t.Run("path to spec ID and back", func(t *testing.T) {
		t.Parallel()

		idMap := newPlaylistIDMap()
		path := "1/my-playlist.m3u"

		// Convert path to spec ID
		specID := idMap.pathToSpecID(path)
		require.Equal(t, specid.Playlist, specID.Type)
		require.NotZero(t, specID.Value)

		// Convert spec ID back to path
		recoveredPath, ok := idMap.specIDToPath(specID)
		require.True(t, ok)
		require.Equal(t, path, recoveredPath)
	})

	t.Run("spec ID to path not found", func(t *testing.T) {
		t.Parallel()

		idMap := newPlaylistIDMap()
		unknownID := specid.ID{Type: specid.Playlist, Value: 99999}

		path, ok := idMap.specIDToPath(unknownID)
		require.False(t, ok)
		require.Empty(t, path)
	})

	t.Run("different paths get different IDs", func(t *testing.T) {
		t.Parallel()

		idMap := newPlaylistIDMap()
		path1 := "1/playlist1.m3u"
		path2 := "1/playlist2.m3u"

		id1 := idMap.pathToSpecID(path1)
		id2 := idMap.pathToSpecID(path2)

		require.NotEqual(t, id1.Value, id2.Value)
	})
}
