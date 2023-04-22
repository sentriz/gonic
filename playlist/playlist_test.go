package playlist_test

import (
	"testing"

	"github.com/matryer/is"
	"go.senan.xyz/gonic/playlist"
)

func TestPlaylist(t *testing.T) {
	is := is.New(t)

	tmp := t.TempDir()
	store, err := playlist.NewStore(tmp)
	is.NoErr(err)

	playlistIDs, err := store.List()
	is.NoErr(err)
	is.True(len(playlistIDs) == 0)

	for _, playlistID := range playlistIDs {
		playlist, err := store.Read(playlistID)
		is.NoErr(err)
		is.True(!playlist.UpdatedAt.IsZero())
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
	is.NoErr(store.Write(newPath, &before))

	after, err := store.Read(newPath)
	is.NoErr(err)

	is.Equal(before.UserID, after.UserID)
	is.Equal(before.Name, after.Name)
	is.Equal(before.Comment, after.Comment)
	is.Equal(before.Items, after.Items)
	is.Equal(before.IsPublic, after.IsPublic)

	playlistIDs, err = store.List()
	is.NoErr(err)
	is.True(len(playlistIDs) == 1)
}
