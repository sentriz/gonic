package lastfm

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble/lastfm/mockclient"
)

func TestScrobble(t *testing.T) {
	// arrange
	t.Parallel()
	require := require.New(t)

	testDB, err := db.NewMock()
	require.NoError(err)
	err = testDB.Migrate(db.MigrationContext{})
	require.NoError(err)

	testDB.SetSetting("lastfm_api_key", "apiKey1")
	testDB.SetSetting("lastfm_secret", "secret1")

	user := &db.User{
		LastFMSession: "lastFMSession1",
	}

	track := &db.Track{
		Album: &db.Album{
			TagTitle: "album1",
			Artists: []*db.Artist{{
				Name: "artist1",
			}},
		},
		Length:         100,
		TagBrainzID:    "916b242d-d439-4ae4-a439-556eef99c06e",
		TagTitle:       "title1",
		TagTrackArtist: "trackArtist1",
		TagTrackNumber: 1,
	}

	stamp := time.Date(2023, 8, 12, 12, 34, 1, 200, time.UTC)

	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodPost, r.Method)
		require.Equal(url.Values{
			"album":       []string{"album1"},
			"albumArtist": []string{"artist1"},
			"api_key":     []string{"apiKey1"},
			"api_sig":     []string{"d235a0b911eb4923953f496c61a2a6af"},
			"artist":      []string{"trackArtist1"},
			"duration":    []string{"100"},
			"method":      []string{"track.Scrobble"},
			"sk":          []string{"lastFMSession1"},
			"mbid":        []string{"916b242d-d439-4ae4-a439-556eef99c06e"},
			"timestamp":   []string{"1691843641"},
			"track":       []string{"title1"},
			"trackNumber": []string{"1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write(mockclient.ArtistGetTopTracksResponse)
	})}

	scrobbler := NewScrobbler(testDB, &client)

	// act
	err = scrobbler.Scrobble(user, track, stamp, true)

	// assert
	require.NoError(err)
}

func TestScrobbleReturnsWithoutLastFMSession(t *testing.T) {
	// arrange
	t.Parallel()
	require := require.New(t)

	scrobbler := Scrobbler{}

	// act
	err := scrobbler.Scrobble(&db.User{}, &db.Track{}, time.Now(), false)

	// assert
	require.NoError(err)
}

func TestScrobbleFailsWithoutLastFMAPIKey(t *testing.T) {
	// arrange
	t.Parallel()
	require := require.New(t)

	testDB, err := db.NewMock()
	require.NoError(err)

	user := &db.User{
		LastFMSession: "lastFMSession1",
	}

	scrobbler := NewScrobbler(testDB, nil)

	// act
	err = scrobbler.Scrobble(user, &db.Track{}, time.Now(), false)

	// assert
	require.Error(err)
}
