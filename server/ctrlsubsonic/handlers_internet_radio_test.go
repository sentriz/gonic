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

const station1id = "ir-1"
var station1ID = specid.ID{Type: specid.InternetRadioStation, Value: 1}
const station1streamURL = "http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"
const station1name = "NRK P1"
const station1homepageURL = "http://www.nrk.no/p1"

const station2id = "ir-2"
var station2ID = specid.ID{Type: specid.InternetRadioStation, Value: 2}
const station2streamURL = "http://lyd.nrk.no/nrk_radio_p2_mp3_m"
const station2name = "NRK P2"
const station2homepageURL = "http://p3.no"

const notaURL = "not_a_url"

const newstation1streamURL = "http://media.kcrw.com/pls/kcrwmusic.pls"
const newstation1name = "KCRW Eclectic"
const newstation1homepageURL = "https://www.kcrw.com/music/shows/eclectic24"

const newstation2streamURL = "http://media.kcrw.com/pls/kcrwsantabarbara.pls"
const newstation2name = "KCRW Santa Barbara"

const station3id = "ir-3"

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

func checkError(t *testing.T, response *spec.SubsonicResponse, code int) {
	if (response.Response.Status != "failed") {
		t.Fatal("didn't return failed status")
	}
	if (response.Response.Error.Code != code) {
		t.Fatal("returned wrong error code")
	}
}

func checkMissingParameter(t *testing.T, response *spec.SubsonicResponse) {
	checkError(t, response, 10)
}

func checkBadParameter(t *testing.T, response *spec.SubsonicResponse) {
	checkError(t, response, 70)
}

func checkNotAdmin(t *testing.T, response *spec.SubsonicResponse) {
	checkError(t, response, 50)
}

func testBadCreates(t *testing.T, contr *Controller) {
	var response *spec.SubsonicResponse
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

	// check for empty get after
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if ((response.Response.InternetRadioStations == nil) ||
			(len(response.Response.InternetRadioStations.List) != 0)) {
		t.Fatal("didn't return empty stations")
	}
}

func testInitialEmpty(t *testing.T, contr *Controller) {
	// check for empty get on new DB
	response := runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if ((response.Response.InternetRadioStations == nil) ||
			(len(response.Response.InternetRadioStations.List) != 0)) {
		t.Fatal("didn't return empty stations")
	}
}

func testInitialAdds(t *testing.T, contr *Controller) {
	// Successful adds and read back
	response := runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station1streamURL},
								 "name": {station1name},
								 "homepageUrl": {station1homepageURL}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station2streamURL},
								 "name": {station2name}}, true) // NOTE: No homepage URL
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station1ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != station1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != station1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != station1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != station2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != station2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != "")) {
		t.Fatal("bad data")
	}
}

func testUpdateHomepage(t *testing.T, contr *Controller) {
	// Update empty homepage URL without other parameters (fails)
	response := runTestCase(t, contr, contr.ServeUpdateInternetRadioStation,
			url.Values{"id": {station2id},
								 "homepageUrl": {station2homepageURL}}, true)
	checkMissingParameter(t, response)

	// Update empty homepage URL properly and read back
	response = runTestCase(t, contr, contr.ServeUpdateInternetRadioStation,
			url.Values{"id": {station2id},
								 "streamUrl": {station2streamURL},
								 "name": {station2name},
								 "homepageUrl": {station2homepageURL}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station1ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != station1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != station1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != station1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != station2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != station2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != station2homepageURL)) {
		t.Fatal("bad data")
	}
}

func testNotAdmin(t *testing.T, contr *Controller) {
	// Create, update, delete w/o admin privileges (fails and does not modify data)
	response := runTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {station1streamURL},
								 "name": {station1name},
								 "homepageUrl": {station1homepageURL}}, false)
	checkNotAdmin(t, response)
	response = runTestCase(t, contr, contr.ServeUpdateInternetRadioStation,
			url.Values{"id": {station1id},
								 "streamUrl": {newstation1streamURL},
								 "name": {newstation1name},
								 "homepageUrl": {newstation1homepageURL}}, false)
	checkNotAdmin(t, response)
	response = runTestCase(t, contr, contr.ServeDeleteInternetRadioStation,
			url.Values{"id": {station1id}}, false)
	checkNotAdmin(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station1ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != station1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != station1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != station1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != station2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != station2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != station2homepageURL)) {
		t.Fatal("bad data")
	}
}

func testUpdates(t *testing.T, contr *Controller) {
	// Replace station 1 and read back
	response := runTestCase(t, contr, contr.ServeUpdateInternetRadioStation,
			url.Values{"id": {station1id},
								 "streamUrl": {newstation1streamURL},
								 "name": {newstation1name},
								 "homepageUrl": {newstation1homepageURL}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station1ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != newstation1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != newstation1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != newstation1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != station2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != station2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != station2homepageURL)) {
		t.Fatal("bad data")
	}

	// Update station 2 but without homepage URL and read back
	response = runTestCase(t, contr, contr.ServeUpdateInternetRadioStation,
			url.Values{"id": {station2id},
								 "streamUrl": {newstation2streamURL},
								 "name": {newstation2name}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station1ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != newstation1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != newstation1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != newstation1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != newstation2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != newstation2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != "")) {
		t.Fatal("bad data")
	}
}

func testDeletes(t *testing.T, contr *Controller) {
	// Delete non-existent station 3 (fails and does not modify data)
	response := runTestCase(t, contr, contr.ServeDeleteInternetRadioStation,
			url.Values{"id": {station3id}}, true)
	checkBadParameter(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 2) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station1ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != newstation1streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != newstation1name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != newstation1homepageURL) ||
			(*response.Response.InternetRadioStations.List[1].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[1].StreamURL != newstation2streamURL) ||
			(response.Response.InternetRadioStations.List[1].Name != newstation2name) ||
			(response.Response.InternetRadioStations.List[1].HomepageURL != "")) {
		t.Fatal("bad data")
	}

	// Delete station 1 and recheck
	response = runTestCase(t, contr, contr.ServeDeleteInternetRadioStation,
			url.Values{"id": {station1id}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
	if (len(response.Response.InternetRadioStations.List) != 1) {
		t.Fatal("wrong number of stations")
	}
	if ((*response.Response.InternetRadioStations.List[0].ID != station2ID) ||
			(response.Response.InternetRadioStations.List[0].StreamURL != newstation2streamURL) ||
			(response.Response.InternetRadioStations.List[0].Name != newstation2name) ||
			(response.Response.InternetRadioStations.List[0].HomepageURL != "")) {
		t.Fatal("bad data")
	}

	// Delete station 2 and check that they're all gone
	response = runTestCase(t, contr, contr.ServeDeleteInternetRadioStation,
			url.Values{"id": {station2id}}, true)
	checkSuccess(t, response)
	response = runTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)
	if ((response.Response.InternetRadioStations == nil) ||
			(len(response.Response.InternetRadioStations.List) != 0)) {
		t.Fatal("didn't return empty stations")
	}
}

func TestInternetRadioStations(t *testing.T) {
	t.Parallel()
	contr := makeController(t)
	
	testInitialEmpty(t, contr)

	testBadCreates(t, contr)

	testInitialAdds(t, contr)

	testUpdateHomepage(t, contr)

	testNotAdmin(t, contr)

	testUpdates(t, contr)

	testDeletes(t, contr)
}