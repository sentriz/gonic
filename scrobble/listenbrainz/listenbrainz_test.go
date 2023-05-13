package listenbrainz

import (
	"context"
	"crypto/tls"
	_ "embed"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.senan.xyz/gonic/db"
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

//go:embed testdata/submit_listens_response.json
var submitListensResponse string

func TestScrobble(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// arrange
	client, close := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Equal("/1/submit-listens", r.URL.Path)
		assert.Equal("application/json", r.Header.Get("Content-Type"))
		assert.Equal("Token token1", r.Header.Get("Authorization"))
		bodyBytes, err := io.ReadAll(r.Body)
		assert.NoError(err)
		assert.JSONEq(submitListensResponse, string(bodyBytes))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted": 1}`))
	}))
	defer close()

	scrobbler := Scrobbler{
		httpClient: &client,
	}

	// act
	err := scrobbler.Scrobble(&db.User{
		ListenBrainzURL:   "https://listenbrainz.org",
		ListenBrainzToken: "token1",
	}, &db.Track{
		Album: &db.Album{
			TagTitle: "album",
		},
		TagTitle:       "title",
		TagTrackArtist: "artist",
		TagTrackNumber: 1,
	}, time.Unix(1683804525, 0), true)

	// assert
	assert.NoError(err)
}

func TestScrobbleUnauthorized(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// arrange
	client, close := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Equal("/1/submit-listens", r.URL.Path)
		assert.Equal("application/json", r.Header.Get("Content-Type"))
		assert.Equal("Token token1", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code": 401, "error": "Invalid authorization token."}`))
	}))
	defer close()

	scrobbler := Scrobbler{
		httpClient: &client,
	}

	// act
	err := scrobbler.Scrobble(&db.User{
		ListenBrainzURL:   "https://listenbrainz.org",
		ListenBrainzToken: "token1",
	}, &db.Track{
		Album: &db.Album{
			TagTitle: "album",
		},
		TagTitle:       "title",
		TagTrackArtist: "artist",
		TagTrackNumber: 1,
	}, time.Now(), true)

	// assert
	assert.ErrorIs(err, ErrListenBrainz)
}

func TestScrobbleServerError(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	// arrange
	client, close := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(http.MethodPost, r.Method)
		assert.Equal("/1/submit-listens", r.URL.Path)
		assert.Equal("application/json", r.Header.Get("Content-Type"))
		assert.Equal("Token token1", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer close()

	scrobbler := Scrobbler{
		httpClient: &client,
	}

	// act
	err := scrobbler.Scrobble(&db.User{
		ListenBrainzURL:   "https://listenbrainz.org",
		ListenBrainzToken: "token1",
	}, &db.Track{
		Album: &db.Album{
			TagTitle: "album",
		},
		TagTitle:       "title",
		TagTrackArtist: "artist",
		TagTrackNumber: 1,
	}, time.Now(), true)

	// assert
	assert.ErrorIs(err, ErrListenBrainz)
}
