package playlist_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/playlist"
)

func TestPlaylist(t *testing.T) {
	require := require.New(t)

	tmp := t.TempDir()
	store, err := playlist.NewStore(tmp)
	require.NoError(err)

	playlistIDs, err := store.List()
	require.NoError(err)
	require.Empty(playlistIDs)

	for _, playlistID := range playlistIDs {
		playlist, err := store.Read(playlistID)
		require.NoError(err)
		require.NotZero(playlist.UpdatedAt)
	}

	before := playlist.Playlist{
		UserID: 10,
		Name:   "Examlpe playlist name",
		Comment: `
Example comment
It has multiple lines 👍
`,
		Items: []string{
			"item 1.flac",
			"item 2.flac",
			"item 3.flac",
		},
		IsPublic: true,
	}

	newPath := playlist.NewPath(before.UserID, before.Name)
	require.NoError(store.Write(newPath, &before))

	after, err := store.Read(newPath)
	require.NoError(err)

	require.Equal(before.UserID, after.UserID)
	require.Equal(before.Name, after.Name)
	require.Equal(before.Comment, after.Comment)
	require.Equal(before.Items, after.Items)
	require.Equal(before.IsPublic, after.IsPublic)

	playlistIDs, err = store.List()
	require.NoError(err)
	require.True(len(playlistIDs) == 1)
}
