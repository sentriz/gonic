//nolint:tparallel,paralleltest,thelper
package ctrlsubsonic

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

const station1ID = "ir-1"

var station1IDT = specid.ID{Type: specid.InternetRadioStation, Value: 1}

const (
	station1StreamURL   = "http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"
	station1Name        = "NRK P1"
	station1HomepageURL = "http://www.nrk.no/p1"
)

const station2ID = "ir-2"

var station2IDT = specid.ID{Type: specid.InternetRadioStation, Value: 2}

const (
	station2StreamURL   = "http://lyd.nrk.no/nrk_radio_p2_mp3_m"
	station2Name        = "NRK P2"
	station2HomepageURL = "http://p3.no"
)

const (
	newstation1StreamURL   = "http://media.kcrw.com/pls/kcrwmusic.pls"
	newstation1Name        = "KCRW Eclectic"
	newstation1HomepageURL = "https://www.kcrw.com/music/shows/eclectic24"
)

const (
	newstation2StreamURL = "http://media.kcrw.com/pls/kcrwsantabarbara.pls"
	newstation2Name      = "KCRW Santa Barbara"
)

const station3ID = "ir-3"

const notAURL = "not_a_url"

func TestInternetRadio(t *testing.T) {
	t.Parallel()

	contr := makeController(t)
	t.Run("initial empty", func(t *testing.T) { testInternetRadioInitialEmpty(t, contr) })
	t.Run("bad creates", func(t *testing.T) { testInternetRadioBadCreates(t, contr) })
	t.Run("initial adds", func(t *testing.T) { testInternetRadioInitialAdds(t, contr) })
	t.Run("update home page", func(t *testing.T) { testInternetRadioUpdateHomepage(t, contr) })
	t.Run("not admin", func(t *testing.T) { testInternetRadioNotAdmin(t, contr) })
	t.Run("updates", func(t *testing.T) { testInternetRadioUpdates(t, contr) })
	t.Run("deletes", func(t *testing.T) { testInternetRadioDeletes(t, contr) })
}

func runTestCase(t *testing.T, h handlerSubsonic, q url.Values, admin bool) *spec.SubsonicResponse {
	t.Helper()

	var rr *httptest.ResponseRecorder
	var req *http.Request

	if admin {
		rr, req = makeHTTPMockWithAdmin(q)
	} else {
		rr, req = makeHTTPMock(q)
	}
	resp(h).ServeHTTP(rr, req)
	body := rr.Body.String()
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("didn't give a 200\n%s", body)
	}

	var response spec.SubsonicResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		var jsonSyntaxError *json.SyntaxError
		if errors.As(err, &jsonSyntaxError) {
			t.Fatalf("invalid character at offset %v\n %s <--", jsonSyntaxError.Offset, body[0:jsonSyntaxError.Offset])
		}

		var jsonUnmarshalTypeError *json.UnmarshalTypeError
		if errors.As(err, &jsonSyntaxError) {
			t.Fatalf("invalid type at offset %v\n %s <--", jsonUnmarshalTypeError.Offset, body[0:jsonUnmarshalTypeError.Offset])
		}

		t.Fatalf("json unmarshal failed: %v", err)
	}

	return &response
}

func checkSuccess(t *testing.T, response *spec.SubsonicResponse) {
	t.Helper()

	if response.Response.Status != "ok" {
		t.Fatal("didn't return ok status")
	}
}

func checkError(t *testing.T, response *spec.SubsonicResponse, code int) {
	t.Helper()

	if response.Response.Status != "failed" {
		t.Fatal("didn't return failed status")
	}
	if response.Response.Error.Code != code {
		t.Fatal("returned wrong error code")
	}
}

func checkMissingParameter(t *testing.T, response *spec.SubsonicResponse) {
	t.Helper()
	checkError(t, response, 10)
}

func checkBadParameter(t *testing.T, response *spec.SubsonicResponse) {
	t.Helper()
	checkError(t, response, 70)
}

func checkNotAdmin(t *testing.T, response *spec.SubsonicResponse) {
	t.Helper()
	checkError(t, response, 50)
}

func testInternetRadioBadCreates(t *testing.T, contr *Controller) {
	var response *spec.SubsonicResponse

	// no parameters
	response = runTestCase(t, contr.ServeCreateInternetRadioStation, url.Values{}, true)
	checkMissingParameter(t, response)

	// just one required parameter
	response = runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"streamUrl": {station1StreamURL}}, true)
	checkMissingParameter(t, response)

	response = runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"name": {station1Name}}, true)
	checkMissingParameter(t, response)

	// bad URLs
	response = runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"streamUrl": {station1StreamURL}, "name": {station1Name}, "homepageUrl": {notAURL}}, true)
	checkBadParameter(t, response)

	response = runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"streamUrl": {notAURL}, "name": {station1Name}, "homepageUrl": {station1HomepageURL}}, true)
	checkBadParameter(t, response)

	// check for empty get after
	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if (response.Response.InternetRadioStations == nil) || (len(response.Response.InternetRadioStations.List) != 0) {
		t.Fatal("didn't return empty stations")
	}
}

func testInternetRadioInitialEmpty(t *testing.T, contr *Controller) {
	// check for empty get on new DB
	response := runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if (response.Response.InternetRadioStations == nil) || (len(response.Response.InternetRadioStations.List) != 0) {
		t.Fatal("didn't return empty stations")
	}
}

func testInternetRadioInitialAdds(t *testing.T, contr *Controller) {
	// successful adds and read back
	response := runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"streamUrl": {station1StreamURL}, "name": {station1Name}, "homepageUrl": {station1HomepageURL}}, true)
	checkSuccess(t, response)

	response = runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"streamUrl": {station2StreamURL}, "name": {station2Name}}, true) // NOTE: no homepage Url
	checkSuccess(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 2 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station1IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != station1StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != station1Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != station1HomepageURL) ||
		(*response.Response.InternetRadioStations.List[1].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[1].StreamURL != station2StreamURL) ||
		(response.Response.InternetRadioStations.List[1].Name != station2Name) ||
		(response.Response.InternetRadioStations.List[1].HomepageURL != "") {
		t.Fatal("bad data")
	}
}

func testInternetRadioUpdateHomepage(t *testing.T, contr *Controller) {
	// update empty homepage URL without other parameters (fails)
	response := runTestCase(t, contr.ServeUpdateInternetRadioStation,
		url.Values{"id": {station2ID}, "homepageUrl": {station2HomepageURL}}, true)
	checkMissingParameter(t, response)

	// update empty homepage URL properly and read back
	response = runTestCase(t, contr.ServeUpdateInternetRadioStation,
		url.Values{"id": {station2ID}, "streamUrl": {station2StreamURL}, "name": {station2Name}, "homepageUrl": {station2HomepageURL}}, true)
	checkSuccess(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 2 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station1IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != station1StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != station1Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != station1HomepageURL) ||
		(*response.Response.InternetRadioStations.List[1].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[1].StreamURL != station2StreamURL) ||
		(response.Response.InternetRadioStations.List[1].Name != station2Name) ||
		(response.Response.InternetRadioStations.List[1].HomepageURL != station2HomepageURL) {
		t.Fatal("bad data")
	}
}

func testInternetRadioNotAdmin(t *testing.T, contr *Controller) {
	// create, update, delete w/o admin privileges (fails and does not modify data)
	response := runTestCase(t, contr.ServeCreateInternetRadioStation,
		url.Values{"streamUrl": {station1StreamURL}, "name": {station1Name}, "homepageUrl": {station1HomepageURL}}, false)
	checkNotAdmin(t, response)

	response = runTestCase(t, contr.ServeUpdateInternetRadioStation,
		url.Values{"id": {station1ID}, "streamUrl": {newstation1StreamURL}, "name": {newstation1Name}, "homepageUrl": {newstation1HomepageURL}}, false)
	checkNotAdmin(t, response)

	response = runTestCase(t, contr.ServeDeleteInternetRadioStation,
		url.Values{"id": {station1ID}}, false)
	checkNotAdmin(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 2 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station1IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != station1StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != station1Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != station1HomepageURL) ||
		(*response.Response.InternetRadioStations.List[1].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[1].StreamURL != station2StreamURL) ||
		(response.Response.InternetRadioStations.List[1].Name != station2Name) ||
		(response.Response.InternetRadioStations.List[1].HomepageURL != station2HomepageURL) {
		t.Fatal("bad data")
	}
}

func testInternetRadioUpdates(t *testing.T, contr *Controller) {
	// replace station 1 and read back
	response := runTestCase(t, contr.ServeUpdateInternetRadioStation,
		url.Values{"id": {station1ID}, "streamUrl": {newstation1StreamURL}, "name": {newstation1Name}, "homepageUrl": {newstation1HomepageURL}}, true)

	checkSuccess(t, response)
	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 2 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station1IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != newstation1StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != newstation1Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != newstation1HomepageURL) ||
		(*response.Response.InternetRadioStations.List[1].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[1].StreamURL != station2StreamURL) ||
		(response.Response.InternetRadioStations.List[1].Name != station2Name) ||
		(response.Response.InternetRadioStations.List[1].HomepageURL != station2HomepageURL) {
		t.Fatal("bad data")
	}

	// update station 2 but without homepage URL and read back
	response = runTestCase(t, contr.ServeUpdateInternetRadioStation,
		url.Values{"id": {station2ID}, "streamUrl": {newstation2StreamURL}, "name": {newstation2Name}}, true)
	checkSuccess(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 2 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station1IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != newstation1StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != newstation1Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != newstation1HomepageURL) ||
		(*response.Response.InternetRadioStations.List[1].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[1].StreamURL != newstation2StreamURL) ||
		(response.Response.InternetRadioStations.List[1].Name != newstation2Name) ||
		(response.Response.InternetRadioStations.List[1].HomepageURL != "") {
		t.Fatal("bad data")
	}
}

func testInternetRadioDeletes(t *testing.T, contr *Controller) {
	// delete non-existent station 3 (fails and does not modify data)
	response := runTestCase(t, contr.ServeDeleteInternetRadioStation,
		url.Values{"id": {station3ID}}, true)
	checkBadParameter(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 2 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station1IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != newstation1StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != newstation1Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != newstation1HomepageURL) ||
		(*response.Response.InternetRadioStations.List[1].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[1].StreamURL != newstation2StreamURL) ||
		(response.Response.InternetRadioStations.List[1].Name != newstation2Name) ||
		(response.Response.InternetRadioStations.List[1].HomepageURL != "") {
		t.Fatal("bad data")
	}

	// delete station 1 and recheck
	response = runTestCase(t, contr.ServeDeleteInternetRadioStation,
		url.Values{"id": {station1ID}}, true)
	checkSuccess(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if response.Response.InternetRadioStations == nil {
		t.Fatal("didn't return stations")
	}
	if len(response.Response.InternetRadioStations.List) != 1 {
		t.Fatal("wrong number of stations")
	}
	if (*response.Response.InternetRadioStations.List[0].ID != station2IDT) ||
		(response.Response.InternetRadioStations.List[0].StreamURL != newstation2StreamURL) ||
		(response.Response.InternetRadioStations.List[0].Name != newstation2Name) ||
		(response.Response.InternetRadioStations.List[0].HomepageURL != "") {
		t.Fatal("bad data")
	}

	// delete station 2 and check that they're all gone
	response = runTestCase(t, contr.ServeDeleteInternetRadioStation,
		url.Values{"id": {station2ID}}, true)
	checkSuccess(t, response)

	response = runTestCase(t, contr.ServeGetInternetRadioStations, url.Values{}, false) // no need to be admin
	checkSuccess(t, response)

	if (response.Response.InternetRadioStations == nil) || (len(response.Response.InternetRadioStations.List) != 0) {
		t.Fatal("didn't return empty stations")
	}
}
