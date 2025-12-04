package ctrlsubsonic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServeGetNowPlaying_FromCache(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	c := Controller{
		nowPlayingCache: NewNowPlayingCache(),
	}

	c.nowPlayingCache.Set(1, NowPlayingRecord{
		TrackID:  101,
		Title:    "Track One",
		IsDir:    false,
		Album:    "Album One",
		Artist:   "Artist One",
		Username: "alice",
		Time:     now.Add(-2 * time.Minute),
		PlayerID: 11,
	})

	c.nowPlayingCache.Set(2, NowPlayingRecord{
		TrackID:  202,
		Title:    "Track Two",
		IsDir:    false,
		Album:    "Album Two",
		Artist:   "Artist Two",
		Username: "bob",
		Time:     now.Add(-30 * time.Minute),
		PlayerID: 22,
	})

	resp := c.ServeGetNowPlaying(nil)
	require.NotNil(t, resp)
	require.NotNil(t, resp.NowPlaying)
	list := resp.NowPlaying.List
	require.Len(t, list, 2)

	// newest first: alice then bob
	first := list[0]
	require.Equal(t, 101, first.Id)
	require.Equal(t, "Track One", first.Title)
	require.Equal(t, "alice", first.Username)
	require.Equal(t, 11, first.PlayerId)
	require.GreaterOrEqual(t, first.MinutesAgo, 0)
	require.LessOrEqual(t, first.MinutesAgo, 5)
	second := list[1]
	require.Equal(t, 202, second.Id)
	require.Equal(t, "Track Two", second.Title)
	require.Equal(t, "bob", second.Username)
	require.Equal(t, 22, second.PlayerId)
	require.GreaterOrEqual(t, second.MinutesAgo, 29)
}
