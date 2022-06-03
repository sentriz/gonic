package ctrlsubsonic

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"net/url"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
  "encoding/json"
)

func RunTestCase(t *testing.T, contr *Controller, h handlerSubsonic, q url.Values, admin bool) (*spec.SubsonicResponse) {
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

func CheckSuccess(t *testing.T, response *spec.SubsonicResponse) {
	if (response.Response.Status != "ok") {
		t.Fatal("didn't return ok status")
	}
}

func CheckMissingParameter(t *testing.T, response *spec.SubsonicResponse) {
	if (response.Response.Status != "failed") {
		t.Fatal("didn't return failed status")
	}
	if (response.Response.Error.Code != 10) {
		t.Fatal("returned wrong error code")
	}
}

func CheckBadParameter(t *testing.T, response *spec.SubsonicResponse) {
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
	
	// Start with some bad creates

	// No parameters
	response := RunTestCase(t, contr, contr.ServeCreateInternetRadioStation, url.Values{}, true)
	CheckMissingParameter(t, response)

	// Just one required parameter
	response = RunTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {"http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"}}, true)
	CheckMissingParameter(t, response)
	response = RunTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"name": {"NRK P1"}}, true)
	CheckMissingParameter(t, response)

	// Bad URLs
	response = RunTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {"http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"},
								 "name": {"NRK P1"},
								 "homepageUrl": {"not_a_url"}}, true)
	CheckBadParameter(t, response)
	response = RunTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {"not_a_url"},
								 "name": {"NRK P1"},
								 "homepageUrl": {"http://www.nrk.no/p1"}}, true)
	CheckBadParameter(t, response)

	// Successful adds and read back
	response = RunTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {"http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"},
								 "name": {"NRK P1"},
								 "homepageUrl": {"http://www.nrk.no/p1"}}, true)
	CheckSuccess(t, response)
	response = RunTestCase(t, contr, contr.ServeCreateInternetRadioStation,
			url.Values{"streamUrl": {"http://lyd.nrk.no/nrk_radio_p2_mp3_m"},
								 "name": {"NRK P2"},
								 "homepageUrl": {"http://p3.no"}}, true)
	CheckSuccess(t, response)
	response = RunTestCase(t, contr, contr.ServeGetInternetRadioStations, url.Values{}, false) //no need to be admin
	CheckSuccess(t, response)
	if (response.Response.InternetRadioStations == nil) {
		t.Fatal("didn't return stations")
	}
}