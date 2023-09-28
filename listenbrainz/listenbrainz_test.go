package listenbrainz_test

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
	"go.senan.xyz/gonic/listenbrainz"
	"go.senan.xyz/gonic/scrobble"
)

func TestScrobble(t *testing.T) {
	t.Parallel()

	client := listenbrainz.NewClientCustom(
		newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/1/submit-listens", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Equal(t, "Token token1", r.Header.Get("Authorization"))
			bodyBytes, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.JSONEq(t, submitListensRequest, string(bodyBytes))

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"accepted": 1}`))
		}),
	)

	err := client.Scrobble(
		db.User{ListenBrainzURL: "https://listenbrainz.org", ListenBrainzToken: "token1"},
		scrobble.Track{Track: "title", Artist: "artist", Album: "album", TrackNumber: 1},
		time.Unix(1683804525, 0),
		true,
	)
	require.NoError(t, err)
}

func TestScrobbleUnauthorized(t *testing.T) {
	t.Parallel()

	client := listenbrainz.NewClientCustom(
		newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/1/submit-listens", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Equal(t, "Token token1", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"code": 401, "error": "Invalid authorization token."}`))
		}),
	)

	err := client.Scrobble(
		db.User{ListenBrainzURL: "https://listenbrainz.org", ListenBrainzToken: "token1"},
		scrobble.Track{Track: "title", Artist: "artist", Album: "album", TrackNumber: 1},
		time.Now(),
		true,
	)

	require.ErrorIs(t, err, listenbrainz.ErrListenBrainz)
}

func TestScrobbleServerError(t *testing.T) {
	t.Parallel()

	client := listenbrainz.NewClientCustom(
		newMockClient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/1/submit-listens", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.Equal(t, "Token token1", r.Header.Get("Authorization"))

			w.WriteHeader(http.StatusInternalServerError)
		}),
	)

	err := client.Scrobble(
		db.User{ListenBrainzURL: "https://listenbrainz.org", ListenBrainzToken: "token1"},
		scrobble.Track{Track: "title", Artist: "artist", Album: "album", TrackNumber: 1},
		time.Now(),
		true,
	)

	require.ErrorIs(t, err, listenbrainz.ErrListenBrainz)
}

func newMockClient(tb testing.TB, handler http.HandlerFunc) *http.Client {
	tb.Helper()

	server := httptest.NewTLSServer(handler)
	tb.Cleanup(server.Close)

	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(network, server.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}
}

//go:embed testdata/submit_listens_request.json
var submitListensRequest string
