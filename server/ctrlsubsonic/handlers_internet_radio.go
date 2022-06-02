package ctrlsubsonic

import (
	"net/http"
	"net/url"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/db"
)

func (c *Controller) ServeGetInternetRadioStations(r *http.Request) *spec.Response {
	var stations []*db.InternetRadioStation
	c.DB.Find(&stations)
	sub := spec.NewResponse()
	sub.InternetRadioStations = &spec.InternetRadioStations{
		List: make([]*spec.InternetRadioStation, len(stations)),
	}
	for i, station := range stations {
		sub.InternetRadioStations.List[i] = spec.NewInternetRadioStation(station)
	}
	return sub
}

func (c *Controller) ServeCreateInternetRadioStation(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}

	params := r.Context().Value(CtxParams).(params.Params)

	streamURL, err := params.Get("streamUrl")
	if err != nil {
		return spec.NewError(10, "no stream URL provided: %s", err)
	}
	_, err = url.ParseRequestURI(streamURL)
	if err != nil {
		return spec.NewError(70, "bad stream URL provided: %s", err)
	}

	name, err := params.Get("name")
	if err != nil {
		return spec.NewError(10, "no name provided: %s", err)
	}

	homepageURL, err := params.Get("homepageUrl")
	if (err == nil) {
			_, err := url.ParseRequestURI(homepageURL)
		if err != nil {
			return spec.NewError(70, "bad homepage URL provided: %s", err)
		}
	}

	var station db.InternetRadioStation
	station.StreamURL = streamURL
	station.Name = name
	station.HomepageURL = homepageURL

	c.DB.Save(&station)

	sub := spec.NewResponse()
	return sub
}

func (c *Controller) ServeUpdateInternetRadioStation(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)

	stationID, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "no id provided: %s", err)
	}

	streamURL, err := params.Get("streamUrl")
	if err != nil {
		return spec.NewError(10, "no stream URL provided: %s", err)
	}
	_, err = url.ParseRequestURI(streamURL)
	if err != nil {
		return spec.NewError(70, "bad stream URL provided: %s", err)
	}

	name, err := params.Get("name")
	if err != nil {
		return spec.NewError(10, "no name provided: %s", err)
	}

	homepageURL, err := params.Get("homepageUrl")
	if (err == nil) {
			_, err := url.ParseRequestURI(homepageURL)
		if err != nil {
			return spec.NewError(70, "bad homepage URL provided: %s", err)
		}
	}

	var station db.InternetRadioStation
	err = c.DB.
		Where("id=?", stationID.Value).
		First(&station).
		Error

	if err != nil {
		return spec.NewError(70, "id not found: %s", err)
	}

	station.StreamURL = streamURL
	station.Name = name
	station.HomepageURL = homepageURL

	c.DB.Save(&station)
	return spec.NewResponse()
}

func (c *Controller) ServeDeleteInternetRadioStation(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)
	
	stationID, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "no id provided: %s", err)
	}

	err = c.DB.
		Where("id=?", stationID.Value).
		Delete(&db.InternetRadioStation{}).
		Error
	if err != nil {
		return spec.NewError(70, "id not found: %s", err)
	}

	return spec.NewResponse()
}
