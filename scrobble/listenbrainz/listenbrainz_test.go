package listenbrainz

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path"
	"regexp"
	"strings"
	"testing"
	"time"

	jd "github.com/josephburnett/jd/lib"
	"github.com/matryer/is"
	"go.senan.xyz/gonic/db"
)

func httpClientMock(handler http.Handler) (http.Client, func()) {
	server := httptest.NewTLSServer(handler)
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(network, server.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	return client, server.Close
}

func TestScrobble(t *testing.T) {
	// arrange
	t.Parallel()
	is := is.NewRelaxed(t)

	client, close := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Equal(http.MethodPost, r.Method)
		is.Equal("/1/submit-listens", r.URL.Path)
		is.Equal("application/json", r.Header.Get("Content-Type"))
		is.Equal("Token token1", r.Header.Get("Authorization"))
		bodyBytes, _ := io.ReadAll(r.Body)
		jsonEqual(t, string(bodyBytes))

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
	is.NoErr(err)
}

func TestScrobble_unauthorized(t *testing.T) {
	// arrange
	t.Parallel()
	is := is.NewRelaxed(t)

	client, close := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Equal(http.MethodPost, r.Method)
		is.Equal("/1/submit-listens", r.URL.Path)
		is.Equal("application/json", r.Header.Get("Content-Type"))
		is.Equal("Token token1", r.Header.Get("Authorization"))

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
	is.True(errors.Is(err, ErrListenBrainz))
}

func TestScrobble_serverError(t *testing.T) {
	// arrange
	t.Parallel()
	is := is.NewRelaxed(t)

	client, close := httpClientMock(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		is.Equal(http.MethodPost, r.Method)
		is.Equal("/1/submit-listens", r.URL.Path)
		is.Equal("application/json", r.Header.Get("Content-Type"))
		is.Equal("Token token1", r.Header.Get("Authorization"))

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
	is.True(errors.Is(err, ErrListenBrainz))
}

func jsonEqual(t *testing.T, body string) bool {
	t.Helper()

	dataPath := getTestDataPath(t.Name())

	expected, err := jd.ReadJsonFile(dataPath)
	if err != nil {
		t.Fatalf("parsing expected: %v", err)
	}
	actual, err := jd.ReadJsonString(body)
	if err != nil {
		t.Fatalf("parsing actual: %v", err)
	}
	diffOpts := []jd.Metadata{}
	diff := expected.Diff(actual, diffOpts...)

	if len(diff) > 0 {
		t.Errorf("\u001b[31;1mactual json differs from expected json\u001b[0m")
		t.Error(diff.Render())
		return false
	}

	return true
}

var testCamelExpr = regexp.MustCompile("([a-z0-9])([A-Z])")

func getTestDataPath(test string) string {
	// convert test name to query case path
	snake := testCamelExpr.ReplaceAllString(test, "${1}_${2}")
	lower := strings.ToLower(snake)
	relPath := strings.ReplaceAll(lower, "/", "_") + ".json"
	return path.Join("testdata", relPath)
}
