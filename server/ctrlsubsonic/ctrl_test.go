package ctrlsubsonic

import (
	"context"
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
	"go.senan.xyz/gonic/server/db"
)

var (
	testDataDir    = "testdata"
	testCamelExpr  = regexp.MustCompile("([a-z0-9])([A-Z])")
	testDBPath     = path.Join(testDataDir, "db")
	testController *Controller
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

func runQueryCases(t *testing.T, h handlerSubsonic, cases []*queryCase) {
	for _, qc := range cases {
		qc := qc // pin
		t.Run(qc.expectPath, func(t *testing.T) {
			t.Parallel()
			rr, req := makeHTTPMock(qc.params)
			testController.H(h).ServeHTTP(rr, req)
			body := rr.Body.String()
			if status := rr.Code; status != http.StatusOK {
				t.Fatalf("didn't give a 200\n%s", body)
			}
			goldenPath := makeGoldenPath(t.Name())
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
			// pass or fail
			if len(diff) == 0 {
				return
			}
			t.Errorf("\u001b[31;1mdiffering json\u001b[0m\n%s", diff.Render())
		})
	}
}

func TestMain(m *testing.M) {
	db, err := db.New(testDBPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	testController = &Controller{
		Controller: &ctrlbase.Controller{DB: db},
	}
	os.Exit(m.Run())
}
