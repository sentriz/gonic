package listenbrainz

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"regexp"
	"strings"
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
		assert.JSONEq(getTestData(t), string(bodyBytes))

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

func TestScrobble_unauthorized(t *testing.T) {
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

func TestScrobble_serverError(t *testing.T) {
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

func getTestData(t *testing.T) string {
	t.Helper()

	dataPath := getTestDataPath(t.Name())
	bytes, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("failed to read test data: %v", err)
	}
	return string(bytes)
}

var testCamelExpr = regexp.MustCompile("([a-z0-9])([A-Z])")

func getTestDataPath(test string) string {
	// convert test name to query case path
	snake := testCamelExpr.ReplaceAllString(test, "${1}_${2}")
	lower := strings.ToLower(snake)
	relPath := strings.ReplaceAll(lower, "/", "_") + ".json"
	return path.Join("testdata", relPath)
}