package playlist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/playlist"
)

func TestPlaylist(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	store, err := playlist.NewStore(tmp, false)
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
			"/item 1.flac",
			"/item 2.flac",
			"/item 3.flac",
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
