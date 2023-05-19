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

	"github.com/stretchr/testify/require"
	"go.senan.xyz/gonic/db"
)

func httpClientMock(handler http.Handler) (http.Client, func()) {
	server := httptest.NewTLSServer(handler)
	shutdown := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(network, server.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}

	return shutdown, server.Close
}

//go:embed testdata/submit_listens_request.json
var submitListensRequest string

func TestScrobble(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// arrange
	client, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodPost, r.Method)
		require.Equal("/1/submit-listens", r.URL.Path)
		require.Equal("application/json", r.Header.Get("Content-Type"))
		require.Equal("Token token1", r.Header.Get("Authorization"))
		bodyBytes, err := io.ReadAll(r.Body)
		require.NoError(err)
		require.JSONEq(submitListensRequest, string(bodyBytes))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accepted": 1}`))
	}))
	defer shutdown()

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
	require.NoError(err)
}

func TestScrobbleUnauthorized(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// arrange
	client, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodPost, r.Method)
		require.Equal("/1/submit-listens", r.URL.Path)
		require.Equal("application/json", r.Header.Get("Content-Type"))
		require.Equal("Token token1", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"code": 401, "error": "Invalid authorization token."}`))
	}))
	defer shutdown()

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
	require.ErrorIs(err, ErrListenBrainz)
}

func TestScrobbleServerError(t *testing.T) {
	t.Parallel()
	require := require.New(t)

	// arrange
	client, shutdown := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(http.MethodPost, r.Method)
		require.Equal("/1/submit-listens", r.URL.Path)
		require.Equal("application/json", r.Header.Get("Content-Type"))
		require.Equal("Token token1", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer shutdown()

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
	require.ErrorIs(err, ErrListenBrainz)
}
