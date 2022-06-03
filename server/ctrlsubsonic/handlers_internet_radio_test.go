package ctrlsubsonic

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"net/url"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
  "encoding/json"
)

const station1streamURL = "http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"
const station1name = "NRK P1"
const station1homepageURL = "http://www.nrk.no/p1"
const station2streamURL = "http://lyd.nrk.no/nrk_radio_p2_mp3_m"
const station2name = "NRK P2"
const station2homepageURL = "http://p3.no"
const notaURL = "not_a_url"

func runTestCase(t *testing.T, contr *Controller, h handlerSubsonic, q url.Values, admin bool) (*spec.SubsonicResponse) {
	var rr *httptest.ResponseRecorder
	var req *http.Request

	if (admin) {
		rr, req = makeHTTPMockWithAdmin(q)
	} else {
  	rr, req = makeHTTPMock(q)
  }
	contr.H(h).ServeHTTP(rr, req)
	body := rr.Body.String()
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("didn't give a 200\n%s", body)
	}

	var response spec.SubsonicResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if (err != nil) {
		switch ty := err.(type) {
		case *json.SyntaxError:
			jsn := body[0:ty.Offset]
      jsn += "<--(Invalid Character)"
      t.Fatalf("Invalid character at offset %v\n %s", ty.Offset, jsn)
		case *json.UnmarshalTypeError:
			jsn := body[0:ty.Offset]
      jsn += "<--(Invalid Type)"
      t.Fatalf("Invalid type at offset %v\n %s", ty.Offset, jsn)
		default:
			t.Fatalf("json unmarshal failed: %s", err.Error())
		}
	}

	return &response
}

func checkSuccess(t *testing.T, response *spec.SubsonicResponse) {
	if (response.Response.Status != "ok") {
		t.Fatal("didn't return ok status")
	}
}

func checkMissingParameter(t *testing.T, response *spec.SubsonicResponse) {
	if (response.Response.Status != "failed") {
		t.Fatal("didn't return failed status")
	}
	if (response.Response.Error.Code != 10) {
		t.Fatal("returned wrong error code")
	}
}

func checkBadParameter(t *testing.T, response *spec.SubsonicResponse) {
	if (response.Response.Status != "failed") {
		t.Fatal("didn't return failed status")
	}
	if (response.Response.Error.Code != 70) {
		t.Fatal("returned wrong error code")
	}
}

func TestInternetRadioStations(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	var response *spec.SubsonicResponse
	
	// check for empty get on new DB
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) //no need to be admin
	checkSuccess(t, response)
	if ((response.Response.InternetRadioStations == nil) ||
			(len(response.Response.InternetRadioStations.List) != 0)) {
		t.Fatal("didn't return empty stations")
	}

	// Bad creates

	// No parameters
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation, url.Values{}, true)
	checkMissingParameter(t, response)

	// Just one required parameter
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station1streamURL}}, true)
	checkMissingParameter(t, response)
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"name": {station1name}}, true)
	checkMissingParameter(t, response)

	// Bad URLs
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station1streamURL},
								 "name": {station1name},
								 "homepageUrl": {notaURL}}, true)
	checkBadParameter(t, response)
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {notaURL},
								 "name": {station1name},
								 "homepageUrl": {station1homepageURL}}, true)
	checkBadParameter(t, response)

	// check for empty get
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) //no need to be admin
	checkSuccess(t, response)
	if ((response.Response.InternetRadioStations == nil) ||
			(len(response.Response.InternetRadioStations.List) != 0)) {
		t.Fatal("didn't return empty stations")
	}

	// Successful adds and read back
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station1streamURL},
								 "name": {station1name},
								 "homepageUrl": {station1homepageURL}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station2streamURL},
								 "name": {station2name}}, true) // NOTE: No homepage URL
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) //no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != specid.ID{specid.InternetRadioStation, 1}) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != station1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != station1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != station1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != specid.ID{specid.InternetRadioStation, 2}) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != station2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != station2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != "")) {
		t.Fatal("bad data")
	}
}