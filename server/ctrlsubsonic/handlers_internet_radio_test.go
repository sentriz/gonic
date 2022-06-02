package ctrlsubsonic

import (
	"testing"
	"net/http"
	"net/url"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
  "encoding/json"
)

func TestInternetRadioStations(t *testing.T) {
	var err error

	t.Parallel()
	contr := makeController(t)
	
	// Start with some bad creates

	// No parameters
	rr, req := makeHTTPMock(url.Values{})
	contr.H(contr.ServeCreateInternetRadioStation).ServeHTTP(rr, req)
	body := rr.Body.String()
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("didn't give a 200\n%s", body)
	}
	var response spec.SubsonicResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)

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