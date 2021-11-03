package ctrlsubsonic

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	jd "github.com/josephburnett/jd/lib"

	"go.senan.xyz/gonic/server/ctrlbase"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/mockfs"
)

var (
	testDataDir   = "testdata"
	testCamelExpr = regexp.MustCompile("([a-z0-9])([A-Z])")
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
	return path.Join(testDataDir, relPath)
}

func makeHTTPMock(query url.Values) (*httptest.ResponseRecorder, *http.Request) {
	// ensure the handlers give us json
	query.Add("f", "json")
	// request from the handler in question
	req, _ := http.NewRequest("", "", nil)
	req.URL.RawQuery = query.Encode()
	subParams := params.New(req)
	withParams := context.WithValue(req.Context(), CtxParams, subParams)
	rr := httptest.NewRecorder()
	req = req.WithContext(withParams)
	return rr, req
}

func runQueryCases(t *testing.T, contr *Controller, h handlerSubsonic, cases []*queryCase) {
	t.Helper()
	for _, qc := range cases {
		t.Run(qc.expectPath, func(t *testing.T) {
			rr, req := makeHTTPMock(qc.params)
			contr.H(h).ServeHTTP(rr, req)
			body := rr.Body.String()
			if status := rr.Code; status != http.StatusOK {
				t.Fatalf("didn't give a 200\n%s", body)
			}

			goldenPath := makeGoldenPath(t.Name())
			goldenRegen := os.Getenv("GONIC_REGEN")
			if goldenRegen == "*" || (goldenRegen != "" && strings.HasPrefix(t.Name(), goldenRegen)) {
				_ = os.WriteFile(goldenPath, []byte(body), 0600)
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
				t.Errorf("\u001b[31;1mdiffering json\u001b[0m\n%s", diff.Render())
			}
		})
	}
}

func makeController(t *testing.T) (*Controller, *mockfs.MockFS)                  { return makec(t, []string{""}) }
func makeControllerRoots(t *testing.T, r []string) (*Controller, *mockfs.MockFS) { return makec(t, r) }

func makec(t *testing.T, roots []string) (*Controller, *mockfs.MockFS) {
	t.Helper()

	m := mockfs.NewWithDirs(t, roots)
	for _, root := range roots {
		m.AddItemsPrefixWithCovers(root)
	}

	m.ScanAndClean()
	m.ResetDates()
	m.LogAlbums()

	base := &ctrlbase.Controller{DB: m.DB()}
	return &Controller{Controller: base}, m
}

func TestMain(m *testing.M) {
	log.SetOutput(ioutil.Discard)
	os.Exit(m.Run())
}
