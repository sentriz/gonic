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
