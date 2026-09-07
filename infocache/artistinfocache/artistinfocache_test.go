package artistinfocache

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/lastfm/mockclient"
	"go.senan.xyz/gonic/mockfs"
)

func TestInfoCache(t *testing.T) {
	t.Parallel()

	m := mockfs.New(t)
	m.AddItems()
	m.ScanAndClean()

	assert := assert.New(t)

	var artist db.Artist
	assert.NoError(m.DB().Find(&artist).Error)
	assert.Greater(artist.ID, 0)
	assert.True(errors.Is(m.DB().First(&db.ArtistInfo{}, "id=?", artist.ID).Error, gorm.ErrRecordNotFound))

	var count atomic.Int32
	lastfmClient := lastfm.NewClientCustom(
		"agent",
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			switch method := r.URL.Query().Get("method"); method {
			case "artist.getInfo":
				count.Add(1)
				w.Write(mockclient.ArtistGetInfoResponse)
			case "artist.getTopTracks":
				w.Write(mockclient.ArtistGetTopTracksResponse)
			}
		}),
		func() (apiKey string, secret string, err error) {
			return "", "", nil
		},
	)

	cache := New(m.DB(), lastfmClient, nil)
	_, err := cache.GetOrLookup(context.Background(), artist.ID)
	require.NoError(t, err)
	_, err = cache.GetOrLookup(context.Background(), artist.ID)
	require.NoError(t, err)

	require.Equal(t, int32(1), count.Load())

	var info db.ArtistInfo
	assert.NoError(m.DB().First(&info, "id=?", artist.ID).Error)
	assert.Equal("Summary", info.LastFMBiography)
}
