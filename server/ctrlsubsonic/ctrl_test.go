//nolint:thelper
package ctrlsubsonic

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	jd "github.com/josephburnett/jd/lib"
	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/mockfs"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/transcode"
)

func TestMain(m *testing.M) {
	gonic.Version = ""
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

var testCamelExpr = regexp.MustCompile("([a-z0-9])([A-Z])")

const (
	mockUsername   = "admin"
	mockPassword   = "admin"
	mockClientName = "test"
)

const (
	audioPath5s  = "testdata/audio/5s.flac"
	audioPath10s = "testdata/audio/10s.flac"
)

type queryCase struct {
	params     url.Values
	expectPath string
	listSet    bool
}

func makeGoldenPath(test string) string {
	// convert test name to query case path
	snake := testCamelExpr.ReplaceAllString(test, "${1}_${2}")
	lower := strings.ToLower(snake)
	relPath := strings.ReplaceAll(lower, "/", "_")
	return filepath.Join("testdata", relPath)
}

func makeHTTPMock(query url.Values) (*httptest.ResponseRecorder, *http.Request) {
	// ensure the handlers give us json
	query.Add("f", "json")
	query.Add("u", mockUsername)
	query.Add("p", mockPassword)
	query.Add("v", "1")
	query.Add("c", mockClientName)
	// request from the handler in question
	req, _ := http.NewRequest("", "", nil)
	req.URL.RawQuery = query.Encode()
	ctx := req.Context()
	ctx = context.WithValue(ctx, CtxParams, params.New(req))
	ctx = context.WithValue(ctx, CtxUser, &db.User{})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	return rr, req
}

func makeHTTPMockWithAdmin(query url.Values) (*httptest.ResponseRecorder, *http.Request) {
	rr, req := makeHTTPMock(query)
	ctx := req.Context()
	ctx = context.WithValue(ctx, CtxUser, &db.User{IsAdmin: true})
	req = req.WithContext(ctx)

	return rr, req
}

func runQueryCases(t *testing.T, h handlerSubsonic, cases []*queryCase) {
	t.Helper()
	for _, qc := range cases {
		t.Run(qc.expectPath, func(t *testing.T) {
			t.Helper()
			t.Parallel()

			rr, req := makeHTTPMock(qc.params)
			resp(h).ServeHTTP(rr, req)
			body := rr.Body.String()
			if status := rr.Code; status != http.StatusOK {
				t.Fatalf("didn't give a 200\n%s", body)
			}

			goldenPath := makeGoldenPath(t.Name())
			goldenRegen := os.Getenv("GONIC_REGEN")
			if goldenRegen == "*" || (goldenRegen != "" && strings.HasPrefix(t.Name(), goldenRegen)) {
				_ = os.WriteFile(goldenPath, []byte(body), 0o600)
				t.Logf("golden file %q regenerated for %s", goldenPath, t.Name())
				t.SkipNow()
			}

			// read case to differ with handler result
			expected, err := jd.ReadJsonFile(goldenPath)
			if err != nil {
				t.Fatalf("parsing expected: %v", err)
			}
			actual, err := jd.ReadJsonString(body)
			if err != nil {
				t.Fatalf("parsing actual: %v", err)
			}
			diffOpts := []jd.Metadata{}
			if qc.listSet {
				diffOpts = append(diffOpts, jd.SET)
			}
			diff := expected.Diff(actual, diffOpts...)

			if len(diff) > 0 {
				t.Errorf("\u001b[31;1mhandler json differs from test json\u001b[0m")
				t.Errorf("\u001b[33;1mif you want to regenerate it, re-run with GONIC_REGEN=%s\u001b[0m\n", t.Name())
				t.Error(diff.Render())
			}
		})
	}
}

func makeController(tb testing.TB) *Controller                  { return makec(tb, []string{""}, false) }
func makeControllerRoots(tb testing.TB, r []string) *Controller { return makec(tb, r, false) }

func makec(tb testing.TB, roots []string, audio bool) *Controller {
	tb.Helper()

	m := mockfs.NewWithDirs(tb, roots)
	for _, root := range roots {
		m.AddItemsPrefixWithCovers(root)
		if !audio {
			continue
		}
		m.SetRealAudio(filepath.Join(root, "artist-0/album-0/track-0.flac"), 10, audioPath10s)
		m.SetRealAudio(filepath.Join(root, "artist-0/album-0/track-1.flac"), 10, audioPath10s)
		m.SetRealAudio(filepath.Join(root, "artist-0/album-0/track-2.flac"), 10, audioPath10s)
	}

	m.ScanAndClean()
	m.ResetDates()

	var absRoots []MusicPath
	for _, root := range roots {
		absRoots = append(absRoots, MusicPath{Path: filepath.Join(m.TmpDir(), root)})
	}

	contr := &Controller{
		dbc:        m.DB(),
		musicPaths: absRoots,
		transcoder: transcode.NewFFmpegTranscoder(),
	}

	return contr
}

func TestParams(t *testing.T) {
	t.Parallel()

	handler := withParams(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := r.Context().Value(CtxParams).(params.Params)
		require.Equal(t, "Client", params.GetOr("c", ""))
	}))
	params := url.Values{}
	params.Set("c", "Client")

	r, err := http.NewRequest(http.MethodGet, "/?"+params.Encode(), nil)
	require.NoError(t, err)
	handler.ServeHTTP(nil, r)

	r, err = http.NewRequest(http.MethodPost, "/", strings.NewReader(params.Encode()))
	require.NoError(t, err)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(nil, r)
}
