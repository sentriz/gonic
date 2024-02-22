//nolint:goconst
package lastfm_test

import (
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/lastfm/mockclient"
	"go.senan.xyz/gonic/scrobble"
)

func TestArtistGetInfo(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{"method": []string{"artist.getInfo"}, "api_key": []string{"apiKey1"}, "artist": []string{"Artist 1"}, "autocorrect": []string{"1"}}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusOK)
			w.Write(mockclient.ArtistGetInfoResponse)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.ArtistGetInfo("Artist 1")
	require.NoError(t, err)
	require.Equal(t, lastfm.Artist{
		XMLName: xml.Name{
			Local: "artist",
		},
		Name:       "Artist 1",
		MBID:       "366c1119-ec4f-4312-b729-a5637d148e3e",
		Streamable: "0",
		Stats: struct {
			Listeners string `xml:"listeners"`
			Playcount string `xml:"playcount"`
		}{
			Listeners: "1",
			Playcount: "2",
		},
		URL: "https://www.last.fm/music/Artist+1",
		Image: []lastfm.Image{
			{
				Size: "small",
				Text: "https://last.fm/artist-1-small.png",
			},
		},
		Bio: lastfm.ArtistBio{
			Published: "13 May 2023, 00:24",
			Summary:   "Summary",
			Content:   "Content",
		},
		Similar: struct {
			Artists []lastfm.Artist `xml:"artist"`
		}{
			Artists: []lastfm.Artist{
				{
					XMLName: xml.Name{
						Local: "artist",
					},
					Name: "Similar Artist 1",
					URL:  "https://www.last.fm/music/Similar+Artist+1",
					Image: []lastfm.Image{
						{
							Size: "small",
							Text: "https://last.fm/similar-artist-1-small.png",
						},
					},
				},
			},
		},
		Tags: struct {
			Tag []lastfm.ArtistTag `xml:"tag"`
		}{
			Tag: []lastfm.ArtistTag{
				{
					Name: "tag1",
					URL:  "https://www.last.fm/tag/tag1",
				},
			},
		},
	}, actual)
}

func TestArtistGetInfoClientRequestFails(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":      []string{"artist.getInfo"},
				"api_key":     []string{"apiKey1"},
				"artist":      []string{"Artist 1"},
				"autocorrect": []string{"1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusInternalServerError)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.ArtistGetInfo("Artist 1")
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestArtistGetTopTracks(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"artist.getTopTracks"},
				"api_key": []string{"apiKey1"},
				"artist":  []string{"artist1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusOK)
			w.Write(mockclient.ArtistGetTopTracksResponse)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.ArtistGetTopTracks("artist1")
	require.NoError(t, err)
	require.Equal(t, lastfm.TopTracks{
		Artist: "Artist 1",
		XMLName: xml.Name{
			Local: "toptracks",
		},
		Tracks: []lastfm.Track{
			{
				Image: []lastfm.Image{
					{
						Text: "https://last.fm/track-1-small.png",
						Size: "small",
					},
					{
						Text: "https://last.fm/track-1-large.png",
						Size: "large",
					},
				},
				Listeners: 2,
				MBID:      "fdfc47cb-69d3-4318-ba71-d54fbc20169a",
				Name:      "Track 1",
				PlayCount: 1,
				Rank:      1,
				URL:       "https://www.last.fm/music/Artist+1/_/Track+1",
			},
			{
				Image: []lastfm.Image{
					{
						Text: "https://last.fm/track-2-small.png",
						Size: "small",
					},
					{
						Text: "https://last.fm/track-2-large.png",
						Size: "large",
					},
				},
				Listeners: 3,
				MBID:      "cf32e694-1ea6-4ba0-9e8b-d5f1950da9c8",
				Name:      "Track 2",
				PlayCount: 2,
				Rank:      2,
				URL:       "https://www.last.fm/music/Artist+1/_/Track+2",
			},
		},
	}, actual)
}

func TestArtistGetTopTracksClientRequestFails(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"artist.getTopTracks"},
				"api_key": []string{"apiKey1"},
				"artist":  []string{"artist1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusInternalServerError)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.ArtistGetTopTracks("artist1")
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestArtistGetSimilar(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"artist.getSimilar"},
				"api_key": []string{"apiKey1"},
				"artist":  []string{"artist1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusOK)
			w.Write(mockclient.ArtistGetSimilarResponse)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.ArtistGetSimilar("artist1")
	require.NoError(t, err)
	require.Equal(t, lastfm.SimilarArtists{
		XMLName: xml.Name{
			Local: "similarartists",
		},
		Artist: "Artist 1",
		Artists: []lastfm.Artist{
			{
				XMLName: xml.Name{
					Local: "artist",
				},
				Image: []lastfm.Image{
					{
						Text: "https://last.fm/artist-2-small.png",
						Size: "small",
					},
					{
						Text: "https://last.fm/artist-2-large.png",
						Size: "large",
					},
				},
				MBID: "d2addad9-3fc4-4ce8-9cd4-63f2a19bb922",
				Name: "Artist 2",
				Similar: struct {
					Artists []lastfm.Artist `xml:"artist"`
				}{},
				Streamable: "0",
				URL:        "https://www.last.fm/music/Artist+2",
			},
			{
				XMLName: xml.Name{
					Local: "artist",
				},
				Image: []lastfm.Image{
					{
						Text: "https://last.fm/artist-3-small.png",
						Size: "small",
					},
					{
						Text: "https://last.fm/artist-3-large.png",
						Size: "large",
					},
				},
				MBID: "dc95d067-df3e-4b83-a5fe-5ec773b1883f",
				Name: "Artist 3",
				Similar: struct {
					Artists []lastfm.Artist `xml:"artist"`
				}{},
				Streamable: "0",
				URL:        "https://www.last.fm/music/Artist+3",
			},
		},
	}, actual)
}

func TestArtistGetSimilarClientRequestFails(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"artist.getSimilar"},
				"api_key": []string{"apiKey1"},
				"artist":  []string{"artist1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusInternalServerError)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.ArtistGetSimilar("artist1")
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestTrackGetSimilarTracks(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"track.getSimilar"},
				"api_key": []string{"apiKey1"},
				"artist":  []string{"artist1"},
				"track":   []string{"track1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusOK)
			w.Write(mockclient.TrackGetSimilarResponse)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.TrackGetSimilarTracks("artist1", "track1")

	require.NoError(t, err)
	require.Equal(t, lastfm.SimilarTracks{
		Artist: "Artist 1",
		Track:  "Track 1",
		XMLName: xml.Name{
			Local: "similartracks",
		},
		Tracks: []lastfm.Track{
			{
				Image: []lastfm.Image{
					{
						Text: "https://last.fm/track-1-small.png",
						Size: "small",
					},
					{
						Text: "https://last.fm/track-1-large.png",
						Size: "large",
					},
				},
				MBID:      "7096931c-bf82-4896-b1e7-42b60a0e16ea",
				Name:      "Track 1",
				PlayCount: 1,
				URL:       "https://www.last.fm/music/Artist+1/_/Track+1",
			},
			{
				Image: []lastfm.Image{
					{
						Text: "https://last.fm/track-2-small.png",
						Size: "small",
					},
					{
						Text: "https://last.fm/track-2-large.png",
						Size: "large",
					},
				},
				MBID:      "2aff1321-149f-4000-8762-3468c917600c",
				Name:      "Track 2",
				PlayCount: 2,
				URL:       "https://www.last.fm/music/Artist+2/_/Track+2",
			},
		},
	}, actual)
}

func TestTrackGetSimilarTracksClientRequestFails(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"track.getSimilar"},
				"api_key": []string{"apiKey1"},
				"artist":  []string{"artist1"},
				"track":   []string{"track1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusInternalServerError)
		}),
		func() (string, string, error) {
			return "apiKey1", "", nil
		},
	)

	actual, err := client.TrackGetSimilarTracks("artist1", "track1")
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestGetSession(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"auth.getSession"},
				"api_key": []string{"apiKey1"},
				"api_sig": []string{"b872a708a0b8b1d9fc1230b1cb6493f8"},
				"token":   []string{"token1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusOK)
			w.Write(mockclient.GetSessionResponse)
		}),
		func() (string, string, error) {
			return "apiKey1", "secret1", nil
		},
	)

	actual, err := client.GetSession("token1")
	require.NoError(t, err)
	require.Equal(t, "sessionKey1", actual)
}

func TestGetSessionClientRequestFails(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, url.Values{
				"method":  []string{"auth.getSession"},
				"api_key": []string{"apiKey1"},
				"api_sig": []string{"b872a708a0b8b1d9fc1230b1cb6493f8"},
				"token":   []string{"token1"},
			}, r.URL.Query())

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusInternalServerError)
		}),
		func() (string, string, error) {
			return "apiKey1", "secret1", nil
		},
	)

	actual, err := client.GetSession("token1")

	require.Error(t, err)
	require.Zero(t, actual)
}

func TestScrobble(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClientCustom(
		mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, url.Values{
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

			require.Equal(t, "/2.0/", r.URL.Path)
			require.Equal(t, lastfm.BaseURL, "https://"+r.Host+r.URL.Path)

			w.WriteHeader(http.StatusOK)
			w.Write(mockclient.ArtistGetTopTracksResponse)
		}),
		func() (apiKey string, secret string, err error) {
			return "apiKey1", "secret1", nil
		},
	)

	user := db.User{
		LastFMSession: "lastFMSession1",
	}
	track := scrobble.Track{
		Track:         "title1",
		Artist:        "trackArtist1",
		Album:         "album1",
		AlbumArtist:   "artist1",
		TrackNumber:   1,
		Duration:      100 * time.Second,
		MusicBrainzID: "916b242d-d439-4ae4-a439-556eef99c06e",
	}

	stamp := time.Date(2023, 8, 12, 12, 34, 1, 200, time.UTC)

	err := client.Scrobble(user, track, stamp, true)
	require.NoError(t, err)
}

func TestScrobbleErrorsWithoutLastFMSession(t *testing.T) {
	t.Parallel()

	client := lastfm.NewClient(func() (apiKey string, secret string, err error) {
		return "", "", nil
	})

	err := client.Scrobble(db.User{}, scrobble.Track{}, time.Now(), false)
	require.ErrorIs(t, err, lastfm.ErrNoUserSession)
}

func TestScrobbleFailsWithoutLastFMAPIKey(t *testing.T) {
	t.Parallel()

	user := db.User{
		LastFMSession: "lastFMSession1",
	}

	scrobbler := lastfm.NewClient(func() (string, string, error) {
		return "", "", fmt.Errorf("no keys")
	})

	err := scrobbler.Scrobble(user, scrobble.Track{}, time.Now(), false)

	require.Error(t, err)
}

func TestGetParamSignature(t *testing.T) {
	t.Parallel()

	params := url.Values{}
	params.Add("ccc", "CCC")
	params.Add("bbb", "BBB")
	params.Add("aaa", "AAA")
	params.Add("ddd", "DDD")
	actual := lastfm.GetParamSignature(params, "secret")
	expected := fmt.Sprintf("%x", md5.Sum([]byte(
		"aaaAAAbbbBBBcccCCCdddDDDsecret",
	)))
	if actual != expected {
		t.Errorf("expected %x, got %s", expected, actual)
	}
}
