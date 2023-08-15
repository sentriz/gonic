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

	require.Equal(after.UserID, before.UserID)
	require.Equal(after.Name, before.Name)
	require.Equal(after.Comment, before.Comment)
	require.Equal(after.Items, before.Items)
	require.Equal(after.IsPublic, before.IsPublic)

	playlistIDs, err = store.List()
	require.NoError(err)
	require.True(len(playlistIDs) == 1)
}
