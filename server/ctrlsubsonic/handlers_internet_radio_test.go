package ctrlsubsonic

import (
	"net/url"
	"testing"
)

const (
	station1StreamURL   = "http://lyd.nrk.no/nrk_radio_p1_ostlandssendingen_mp3_m"
	station1Name        = "NRK P1"
	station1HomepageURL = "http://www.nrk.no/p1"
	station2StreamURL   = "http://lyd.nrk.no/nrk_radio_p2_mp3_m"
	station2Name        = "NRK P2"
	station2HomepageURL = "http://p3.no"

	newStation1StreamURL   = "http://media.kcrw.com/pls/kcrwmusic.pls"
	newStation1Name        = "KCRW Eclectic"
	newStation1HomepageURL = "https://www.kcrw.com/music/shows/eclectic24"
	newStation2StreamURL   = "http://media.kcrw.com/pls/kcrwsantabarbara.pls"
	newStation2Name        = "KCRW Santa Barbara"

	notAURL = "not_a_url"
)

// state is shared across the whole test (created stations carry their auto-
// increment id forward), so seq=true and each step golden-tests the state
// after the mutation.
func TestInternetRadio(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.seq = true

	get := func(name string) {
		t.Helper()
		f.run(t, f.contr.ServeGetInternetRadioStations, f.admin,
			query{url.Values{}, name, false},
		)
	}

	get("initial_empty")

	// admin: create rejected without required params or with bad URLs.
	f.run(t, f.contr.ServeCreateInternetRadioStation, f.admin,
		query{url.Values{}, "create_no_params", false},
		query{url.Values{"streamUrl": {station1StreamURL}}, "create_no_name", false},
		query{url.Values{"name": {station1Name}}, "create_no_stream_url", false},
		query{url.Values{"streamUrl": {station1StreamURL}, "name": {station1Name}, "homepageUrl": {notAURL}}, "create_bad_homepage", false},
		query{url.Values{"streamUrl": {notAURL}, "name": {station1Name}, "homepageUrl": {station1HomepageURL}}, "create_bad_stream", false},
	)
	get("after_bad_creates")

	// admin: successful adds.
	f.run(t, f.contr.ServeCreateInternetRadioStation, f.admin,
		query{url.Values{"streamUrl": {station1StreamURL}, "name": {station1Name}, "homepageUrl": {station1HomepageURL}}, "create_station1", false},
		query{url.Values{"streamUrl": {station2StreamURL}, "name": {station2Name}}, "create_station2_no_homepage", false},
	)
	get("after_initial_adds")

	// update: missing required params should still reject (homepage alone isn't enough).
	f.run(t, f.contr.ServeUpdateInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-2"}, "homepageUrl": {station2HomepageURL}}, "update_homepage_only_rejected", false},
	)
	// full update of station 2.
	f.run(t, f.contr.ServeUpdateInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-2"}, "streamUrl": {station2StreamURL}, "name": {station2Name}, "homepageUrl": {station2HomepageURL}}, "update_station2_full", false},
	)
	get("after_update_homepage")

	// non-admin: every mutation must be refused (code 50) and leave state intact.
	f.run(t, f.contr.ServeCreateInternetRadioStation, f.alt,
		query{url.Values{"streamUrl": {station1StreamURL}, "name": {station1Name}, "homepageUrl": {station1HomepageURL}}, "create_not_admin", false},
	)
	f.run(t, f.contr.ServeUpdateInternetRadioStation, f.alt,
		query{url.Values{"id": {"ir-1"}, "streamUrl": {newStation1StreamURL}, "name": {newStation1Name}, "homepageUrl": {newStation1HomepageURL}}, "update_not_admin", false},
	)
	f.run(t, f.contr.ServeDeleteInternetRadioStation, f.alt,
		query{url.Values{"id": {"ir-1"}}, "delete_not_admin", false},
	)
	get("after_not_admin_attempts")

	// admin: replace station 1 in full.
	f.run(t, f.contr.ServeUpdateInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-1"}, "streamUrl": {newStation1StreamURL}, "name": {newStation1Name}, "homepageUrl": {newStation1HomepageURL}}, "replace_station1", false},
	)
	get("after_replace_station1")

	// admin: update station 2, drop the homepage URL by omitting it.
	f.run(t, f.contr.ServeUpdateInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-2"}, "streamUrl": {newStation2StreamURL}, "name": {newStation2Name}}, "drop_station2_homepage", false},
	)
	get("after_drop_homepage")

	// admin: delete a non-existent station, then delete the existing ones.
	f.run(t, f.contr.ServeDeleteInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-3"}}, "delete_missing", false},
	)
	get("after_delete_missing")
	f.run(t, f.contr.ServeDeleteInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-1"}}, "delete_station1", false},
	)
	get("after_delete_station1")
	f.run(t, f.contr.ServeDeleteInternetRadioStation, f.admin,
		query{url.Values{"id": {"ir-2"}}, "delete_station2", false},
	)
	get("after_delete_station2")
}
