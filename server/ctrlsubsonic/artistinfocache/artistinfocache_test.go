package artistinfocache

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mockfs"
	"go.senan.xyz/gonic/scrobble/lastfm"
	"go.senan.xyz/gonic/scrobble/lastfm/mockclient"
)

func TestInfoCache(t *testing.T) {
	t.Parallel()

	m := mockfs.New(t)
	m.AddItems()
	m.ScanAndClean()

	assert := assert.New(t)

	var artist db.Artist
	assert.NoError(m.DB().Preload("Info").Find(&artist).Error)
	assert.Greater(artist.ID, 0)
	assert.Nil(artist.Info)

	var count atomic.Int32
	lastfmClient := lastfm.NewClientCustom(mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		switch method := r.URL.Query().Get("method"); method {
		case "artist.getInfo":
			count.Add(1)
			w.Write(mockclient.ArtistGetInfoResponse)
		case "artist.getTopTracks":
			w.Write(mockclient.ArtistGetTopTracksResponse)
		}
	}))

	cache := New(m.DB(), lastfmClient)
	_, err := cache.GetOrLookup(context.Background(), "", artist.ID)
	require.NoError(t, err)
	_, err = cache.GetOrLookup(context.Background(), "", artist.ID)
	require.NoError(t, err)

	require.Equal(t, int32(1), count.Load())

	assert.NoError(m.DB().Preload("Info").Find(&artist, "id=?", artist.ID).Error)
	assert.Greater(artist.ID, 0)
	assert.NotNil(artist.Info)
	assert.Equal("Summary", artist.Info.Biography)
}
