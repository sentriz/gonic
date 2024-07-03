package playlist_test

import (
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
			"item 1.flac",
			"item 2.flac",
			"item 3.flac",
		},
		IsPublic:   true,
		SharedWith: []int{2, 3},
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
	require.Equal(t, after.SharedWith, before.SharedWith)

	playlistIDs, err = store.List()
	require.NoError(t, err)
	require.True(t, len(playlistIDs) == 1)
}

func TestAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		playlist     playlist.Playlist
		userID       int
		expectRead   bool
		expectWrite  bool
		expectDelete bool
	}{
		{
			name: "owner can do anything",
			playlist: playlist.Playlist{
				UserID: 7,
			},
			userID:       7,
			expectRead:   true,
			expectWrite:  true,
			expectDelete: true,
		},
		{
			name: "third party cannot do anything",
			playlist: playlist.Playlist{
				UserID: 7,
			},
			userID:       99,
			expectRead:   false,
			expectWrite:  false,
			expectDelete: false,
		},
		{
			name: "third party can read if public",
			playlist: playlist.Playlist{
				IsPublic: true,
				UserID:   7,
			},
			userID:       99,
			expectRead:   true,
			expectWrite:  false,
			expectDelete: false,
		},
		{
			name: "shared user can read and write",
			playlist: playlist.Playlist{
				IsPublic:   true,
				UserID:     7,
				SharedWith: []int{99},
			},
			userID:       99,
			expectRead:   true,
			expectWrite:  true,
			expectDelete: false,
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.expectRead, tc.playlist.CanRead(tc.userID))
			require.Equal(t, tc.expectWrite, tc.playlist.CanWrite(tc.userID))
			require.Equal(t, tc.expectDelete, tc.playlist.CanDelete(tc.userID))
		})
	}
}
