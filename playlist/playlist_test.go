package playlist_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/gonic/playlist"
)

func TestPlaylist(t *testing.T) {
	assert := assert.New(t)

	tmp := t.TempDir()
	store, err := playlist.NewStore(tmp)
	assert.NoError(err)

	playlistIDs, err := store.List()
	assert.NoError(err)
	assert.Empty(playlistIDs)

	for _, playlistID := range playlistIDs {
		playlist, err := store.Read(playlistID)
		assert.NoError(err)
		assert.NotZero(playlist.UpdatedAt)
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
	assert.NoError(store.Write(newPath, &before))

	after, err := store.Read(newPath)
	assert.NoError(err)

	assert.Equal(before.UserID, after.UserID)
	assert.Equal(before.Name, after.Name)
	assert.Equal(before.Comment, after.Comment)
	assert.Equal(before.Items, after.Items)
	assert.Equal(before.IsPublic, after.IsPublic)

	playlistIDs, err = store.List()
	assert.NoError(err)
	assert.True(len(playlistIDs) == 1)
}
