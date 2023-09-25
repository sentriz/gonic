package lastfm

import (
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/scrobble/lastfm/mockclient"
)

func TestArtistGetInfo(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"artist.getInfo"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"Artist 1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write(mockclient.ArtistGetInfoResponse)
	})}

	// act
	actual, err := client.ArtistGetInfo("apiKey1", "Artist 1")

	// assert
	require.NoError(t, err)
	require.Equal(t, Artist{
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
		Image: []Image{
			{
				Size: "small",
				Text: "https://last.fm/artist-1-small.png",
			},
		},
		Bio: ArtistBio{
			Published: "13 May 2023, 00:24",
			Summary:   "Summary",
			Content:   "Content",
		},
		Similar: struct {
			Artists []Artist `xml:"artist"`
		}{
			Artists: []Artist{
				{
					XMLName: xml.Name{
						Local: "artist",
					},
					Name: "Similar Artist 1",
					URL:  "https://www.last.fm/music/Similar+Artist+1",
					Image: []Image{
						{
							Size: "small",
							Text: "https://last.fm/similar-artist-1-small.png",
						},
					},
				},
			},
		},
		Tags: struct {
			Tag []ArtistTag `xml:"tag"`
		}{
			Tag: []ArtistTag{
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

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"artist.getInfo"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"Artist 1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	})}

	// act
	actual, err := client.ArtistGetInfo("apiKey1", "Artist 1")

	// assert
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestArtistGetTopTracks(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"artist.getTopTracks"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write(mockclient.ArtistGetTopTracksResponse)
	})}

	// act
	actual, err := client.ArtistGetTopTracks("apiKey1", "artist1")

	// assert
	require.NoError(t, err)
	require.Equal(t, TopTracks{
		Artist: "Artist 1",
		XMLName: xml.Name{
			Local: "toptracks",
		},
		Tracks: []Track{
			{
				Image: []Image{
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
				Image: []Image{
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

func TestArtistGetTopTracks_clientRequestFails(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"artist.getTopTracks"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	})}

	// act
	actual, err := client.ArtistGetTopTracks("apiKey1", "artist1")

	// assert
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestArtistGetSimilar(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"artist.getSimilar"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write(mockclient.ArtistGetSimilarResponse)
	})}

	// act
	actual, err := client.ArtistGetSimilar("apiKey1", "artist1")

	// assert
	require.NoError(t, err)
	require.Equal(t, SimilarArtists{
		XMLName: xml.Name{
			Local: "similarartists",
		},
		Artist: "Artist 1",
		Artists: []Artist{
			{
				XMLName: xml.Name{
					Local: "artist",
				},
				Image: []Image{
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
					Artists []Artist `xml:"artist"`
				}{},
				Streamable: "0",
				URL:        "https://www.last.fm/music/Artist+2",
			},
			{
				XMLName: xml.Name{
					Local: "artist",
				},
				Image: []Image{
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
					Artists []Artist `xml:"artist"`
				}{},
				Streamable: "0",
				URL:        "https://www.last.fm/music/Artist+3",
			},
		},
	}, actual)
}

func TestArtistGetSimilar_clientRequestFails(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"artist.getSimilar"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	})}

	// act
	actual, err := client.ArtistGetSimilar("apiKey1", "artist1")

	// assert
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestTrackGetSimilarTracks(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"track.getSimilar"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
			"track":   []string{"track1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write(mockclient.TrackGetSimilarResponse)
	})}

	// act
	actual, err := client.TrackGetSimilarTracks("apiKey1", "artist1", "track1")

	// assert
	require.NoError(t, err)
	require.Equal(t, SimilarTracks{
		Artist: "Artist 1",
		Track:  "Track 1",
		XMLName: xml.Name{
			Local: "similartracks",
		},
		Tracks: []Track{
			{
				Image: []Image{
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
				Image: []Image{
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

func TestTrackGetSimilarTracks_clientRequestFails(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"track.getSimilar"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
			"track":   []string{"track1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	})}

	// act
	actual, err := client.TrackGetSimilarTracks("apiKey1", "artist1", "track1")

	// assert
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestGetSession(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"auth.getSession"},
			"api_key": []string{"apiKey1"},
			"api_sig": []string{"b872a708a0b8b1d9fc1230b1cb6493f8"},
			"token":   []string{"token1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write(mockclient.GetSessionResponse)
	})}

	// act
	actual, err := client.GetSession("apiKey1", "secret1", "token1")

	// assert
	require.NoError(t, err)
	require.Equal(t, "sessionKey1", actual)
}

func TestGetSessioeClientRequestFails(t *testing.T) {
	t.Parallel()

	// arrange
	client := Client{mockclient.New(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, url.Values{
			"method":  []string{"auth.getSession"},
			"api_key": []string{"apiKey1"},
			"api_sig": []string{"b872a708a0b8b1d9fc1230b1cb6493f8"},
			"token":   []string{"token1"},
		}, r.URL.Query())

		require.Equal(t, "/2.0/", r.URL.Path)
		require.Equal(t, baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	})}

	// act
	actual, err := client.GetSession("apiKey1", "secret1", "token1")

	// assert
	require.Error(t, err)
	require.Zero(t, actual)
}

func TestGetParamSignature(t *testing.T) {
	t.Parallel()

	params := url.Values{}
	params.Add("ccc", "CCC")
	params.Add("bbb", "BBB")
	params.Add("aaa", "AAA")
	params.Add("ddd", "DDD")
	actual := getParamSignature(params, "secret")
	expected := fmt.Sprintf("%x", md5.Sum([]byte(
		"aaaAAAbbbBBBcccCCCdddDDDsecret",
	)))
	if actual != expected {
		t.Errorf("expected %x, got %s", expected, actual)
	}
}
