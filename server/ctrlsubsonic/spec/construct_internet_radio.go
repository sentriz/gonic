package spec

import "go.senan.xyz/gonic/db"

func NewInternetRadioStation(irs *db.InternetRadioStation) *InternetRadioStation {
	return &InternetRadioStation{
		ID:          irs.SID(),
		Name:        irs.Name,
		StreamURL:   irs.StreamURL,
		HomepageURL: irs.HomepageURL,
	}
}

func NewTCInternetRadioStation(irs *db.InternetRadioStation) *TrackChild {
	return &TrackChild{
		ID:      irs.SID(),
		Title:   irs.Name,
		IsDir:   false,
		IsVideo: false,
		Type:    "internetradio",
	}
}
