package ctrlsubsonic

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"net/url"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
  "encoding/json"
)

func RunTestCase(t *testing.T, contr *Controller, h handlerSubsonic, q url.Values) (*httptest.ResponseRecorder) {
	rr, req := makeHTTPMock(q)
	contr.H(h).ServeHTTP(rr, req)
	body := rr.Body.String()
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("didn't give a 200\n%s", body)
	}
	return rr
}

func CheckMissingParameter(t *testing.T, data []byte) {
	var response spec.SubsonicResponse
	err := json.Unmarshal(data, &response)
	if (err != nil) {
		t.Fatal("json parsing failed")
	}
	if (response.Response.Status != "failed") {
		t.Fatal("didn't return failed status")
	}
	if (response.Response.Error.Code != 10) {
		t.Fatal("returned wrong error code")
	}
}

func TestInternetRadioStations(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	
	// Start with some bad creates

	// No parameters
	rr := RunTestCase(t, contr, contr.ServeCreateInternetRadioStation, url.Values{})
	CheckMissingParameter(t, rr.Body.Bytes())
}