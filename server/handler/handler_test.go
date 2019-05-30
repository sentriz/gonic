package handler

import (
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/jinzhu/gorm"
	jd "github.com/josephburnett/jd/lib"
)

var (
	testController *Controller
	testDataDir    = "test_data"
	testCamelExpr  = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func init() {
	testDBPath := path.Join(testDataDir, "db")
	testDB, err := gorm.Open("sqlite3", testDBPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	testController = &Controller{DB: testDB}
}

type queryCase struct {
	params     url.Values
	expectPath string
	listSet    bool
}

func testNameToPath(name string) string {
	snake := testCamelExpr.ReplaceAllString(name, "${1}_${2}")
	lower := strings.ToLower(snake)
	relPath := strings.Replace(lower, "/", "_", -1)
	return path.Join(testDataDir, relPath)
}

func testQueryCases(t *testing.T, handler http.HandlerFunc, cases []*queryCase) {
	for _, qc := range cases {
		t.Run(qc.expectPath, func(t *testing.T) {
			// ensure the handlers give us json
			qc.params.Add("f", "json")
			req, _ := http.NewRequest("", "?"+qc.params.Encode(), nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			body := rr.Body.String()
			if status := rr.Code; status != http.StatusOK {
				t.Fatalf("didn't give a 200\n%s", body)
			}
			absExpPath := testNameToPath(t.Name())
			expected, err := jd.ReadJsonFile(absExpPath)
			if err != nil {
				t.Fatalf("parsing expected: %v", err)
			}
			actual, _ := jd.ReadJsonString(body)
			if err != nil {
				t.Fatalf("parsing actual: %v", err)
			}
			diffOpts := []jd.Metadata{}
			if qc.listSet {
				diffOpts = append(diffOpts, jd.SET)
			}
			diff := expected.Diff(actual, diffOpts...)
			if len(diff) == 0 {
				return
			}
			t.Errorf("\u001b[31;1mdiffering json\u001b[0m\n%s",
				diff.Render())
		})
	}
}
