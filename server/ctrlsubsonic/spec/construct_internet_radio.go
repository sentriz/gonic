package spec

import "go.senan.xyz/gonic/db"

func NewInternetRadioStation(p *db.InternetRadioStation) *InternetRadioStation {
	ret := &InternetRadioStation{
		ID:				p.SID(),
		Name:				p.Name,
		StreamURL:			p.StreamURL,
		HomepageURL:			p.HomepageURL,
	}
	return ret
}
