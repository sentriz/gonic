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

func TestGetParamSignature(t *testing.T) {
	// arrange
	params := url.Values{
		"aaa": []string{"AAA"},
		"bbb": []string{"BBB"},
		"ccc": []string{"CCC"},
		"ddd": []string{"DDD"},
	}
	client := Client{"apiKey", "secret", nil}

	// act
	actual := client.getParamSignature(params)

	// assert
	expected := fmt.Sprintf("%x", md5.Sum([]byte(
		"aaaAAAbbbBBBcccCCCdddDDDsecret",
	)))
	require.Equal(t, expected, actual)
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
			"api_key": []string{"apiKey"},
			"artist":  []string{"artist"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(artistGetInfoResponse))
	}))
	defer shutdown()

	client := Client{"apiKey", "secret", &httpClient}

	// act
	actual, err := client.ArtistGetInfo("artist")

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
		Image: []ArtistImage{
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
					Image: []ArtistImage{
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
			"api_key": []string{"apiKey"},
			"artist":  []string{"artist"},
		}, r.URL.Query())
		require.Equal("/2.0/", r.URL.Path)
		require.Equal(baseURL, "https://"+r.Host+r.URL.Path)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer shutdown()

	client := Client{"apiKey", "secret", &httpClient}

	// act
	actual, err := client.ArtistGetInfo("artist")

	// assert
	require.Error(err)
	require.Zero(actual)
}
