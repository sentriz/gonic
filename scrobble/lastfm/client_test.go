package lastfm

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	_ "embed"
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func httpClientMock(handler http.Handler) (http.Client, func()) {
	server := httptest.NewTLSServer(handler)
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(network, server.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}

	return client, server.Close
}

//go:embed testdata/artist_get_info_response.xml
var artistGetInfoResponse string

func TestArtistGetInfo(t *testing.T) {
	// arrange
	require := require.New(t)
	httpClient, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodGet, r.Method)
		require.Equal(url.Values{
			"method":  []string{"artist.getInfo"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"Artist 1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(artistGetInfoResponse))
	}))
	defer shutdown()

	client := Client{&httpClient}

	// act
	actual, err := client.ArtistGetInfo("apiKey1", "Artist 1")

	// assert
	require.NoError(err)
	require.Equal(Artist{
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

func TestArtistGetInfo_clientRequestFails(t *testing.T) {
	// arrange
	require := require.New(t)
	httpClient, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodGet, r.Method)
		require.Equal(url.Values{
			"method":  []string{"artist.getInfo"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"Artist 1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer shutdown()

	client := Client{&httpClient}

	// act
	actual, err := client.ArtistGetInfo("apiKey1", "Artist 1")

	// assert
	require.Error(err)
	require.Zero(actual)
}

//go:embed testdata/artist_get_top_tracks_response.xml
var artistGetTopTracksResponse string

func TestArtistGetTopTracks(t *testing.T) {
	// arrange
	require := require.New(t)
	httpClient, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodGet, r.Method)
		require.Equal(url.Values{
			"method":  []string{"artist.getTopTracks"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(artistGetTopTracksResponse))
	}))
	defer shutdown()

	client := Client{&httpClient}

	// act
	actual, err := client.ArtistGetTopTracks("apiKey1", "artist1")

	// assert
	require.NoError(err)
	require.Equal(TopTracks{
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
	// arrange
	require := require.New(t)
	httpClient, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodGet, r.Method)
		require.Equal(url.Values{
			"method":  []string{"artist.getTopTracks"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer shutdown()

	client := Client{&httpClient}

	// act
	actual, err := client.ArtistGetTopTracks("apiKey1", "artist1")

	// assert
	require.Error(err)
	require.Zero(actual)
}

//go:embed testdata/track_get_similar_response.xml
var trackGetSimilaResponse string

func TestTrackGetSimilarTracks(t *testing.T) {
	// arrange
	require := require.New(t)
	httpClient, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodGet, r.Method)
		require.Equal(url.Values{
			"method":  []string{"track.getSimilar"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
			"track":   []string{"track1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(trackGetSimilaResponse))
	}))
	defer shutdown()

	client := Client{&httpClient}

	// act
	actual, err := client.TrackGetSimilarTracks("apiKey1", "artist1", "track1")

	// assert
	require.NoError(err)
	require.Equal(SimilarTracks{
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
	// arrange
	require := require.New(t)
	httpClient, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodGet, r.Method)
		require.Equal(url.Values{
			"method":  []string{"track.getSimilar"},
			"api_key": []string{"apiKey1"},
			"artist":  []string{"artist1"},
			"track":   []string{"track1"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer shutdown()

	client := Client{&httpClient}

	// act
	actual, err := client.TrackGetSimilarTracks("apiKey1", "artist1", "track1")

	// assert
	require.Error(err)
	require.Zero(actual)
}

func TestGetParamSignature(t *testing.T) {
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
