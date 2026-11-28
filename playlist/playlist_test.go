package playlist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/playlist"
)

func TestPlaylist(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	store, err := playlist.NewStore(tmp)
	require.NoError(t, err)

	playlistIDs, err := store.List()
	require.NoError(t, err)
	require.Empty(t, playlistIDs)

	for _, playlistID := range playlistIDs {
		playlist, err := store.Read(playlistID)
		require.NoError(t, err)
		require.NotZero(t, playlist.UpdatedAt)
	}

	before := playlist.Playlist{
		UserID: 10,
		Name:   "Examlpe playlist name",
		Comment: `
Example comment
It has multiple lines 👍
`,
		Items: []string{
			"/music/item 1.flac",
			"/music/item 2.flac",
			"/music/item 3.flac",
		},
		IsPublic: true,
	}

	newPath := playlist.NewPath(before.UserID, before.Name)
	require.NoError(t, store.Write(newPath, &before))

	after, err := store.Read(newPath)
	require.NoError(t, err)

	require.Equal(t, after.UserID, before.UserID)
	require.Equal(t, after.Name, before.Name)
	require.Equal(t, after.Comment, before.Comment)
	require.Equal(t, after.Items, before.Items)
	require.Equal(t, after.IsPublic, before.IsPublic)

	playlistIDs, err = store.List()
	require.NoError(t, err)
	require.True(t, len(playlistIDs) == 1)
}

func TestPlaylistCoverPath(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	store, err := playlist.NewStore(tmp)
	require.NoError(t, err)

	files := []string{
		"1/a.m3u", "1/a.jpg",
		"1/b.m3u", "1/b.png",
		"1/c.m3u",
		"1/sub/d.m3u", "1/sub/d.jpg",
	}
	for _, f := range files {
		path := filepath.Join(tmp, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, nil, 0o600))
	}

	for relPath, coverPath := range map[string]string{
		"1/a.m3u":     "1/a.jpg",
		"1/b.m3u":     "1/b.png",
		"1/c.m3u":     "",
		"1/sub/d.m3u": "1/sub/d.jpg",
	} {
		pl, err := store.Read(relPath)
		require.NoError(t, err)
		if coverPath != "" {
			coverPath = filepath.Join(tmp, coverPath)
		}
		require.Equal(t, coverPath, pl.CoverPath, relPath)
	}
}
