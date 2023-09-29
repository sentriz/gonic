package ctrlsubsonic

import (
	"net/http"
	"net/url"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

func (c *Controller) ServeGetInternetRadioStations(_ *http.Request) *spec.Response {
	var stations []*db.InternetRadioStation
	if err := c.dbc.Find(&stations).Error; err != nil {
		return spec.NewError(0, "find stations: %v", err)
	}
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
	if !user.IsAdmin {
		return spec.NewError(50, "user not admin")
	}

	params := r.Context().Value(CtxParams).(params.Params)

	streamURL, err := params.Get("streamUrl")
	if err != nil {
		return spec.NewError(10, "no stream URL provided: %v", err)
	}
	if _, err := url.ParseRequestURI(streamURL); err != nil {
		return spec.NewError(70, "bad stream URL provided: %v", err)
	}
	name, err := params.Get("name")
	if err != nil {
		return spec.NewError(10, "no name provided: %v", err)
	}
	homepageURL, err := params.Get("homepageUrl")
	if err == nil && homepageURL != "" {
		if _, err := url.ParseRequestURI(homepageURL); err != nil {
			return spec.NewError(70, "bad homepage URL provided: %v", err)
		}
	}

	var station db.InternetRadioStation
	station.StreamURL = streamURL
	station.Name = name
	station.HomepageURL = homepageURL

	if err := c.dbc.Save(&station).Error; err != nil {
		return spec.NewError(0, "save station: %v", err)
	}

	return spec.NewResponse()
}

func (c *Controller) ServeUpdateInternetRadioStation(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if !user.IsAdmin {
		return spec.NewError(50, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)

	stationID, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "no id provided: %v", err)
	}
	streamURL, err := params.Get("streamUrl")
	if err != nil {
		return spec.NewError(10, "no stream URL provided: %v", err)
	}
	if _, err = url.ParseRequestURI(streamURL); err != nil {
		return spec.NewError(70, "bad stream URL provided: %v", err)
	}
	name, err := params.Get("name")
	if err != nil {
		return spec.NewError(10, "no name provided: %v", err)
	}
	homepageURL, err := params.Get("homepageUrl")
	if err == nil {
		if _, err := url.ParseRequestURI(homepageURL); err != nil {
			return spec.NewError(70, "bad homepage URL provided: %v", err)
		}
	}

	var station db.InternetRadioStation
	if err := c.dbc.Where("id=?", stationID.Value).First(&station).Error; err != nil {
		return spec.NewError(70, "id not found: %v", err)
	}

	station.StreamURL = streamURL
	station.Name = name
	station.HomepageURL = homepageURL

	if err := c.dbc.Save(&station).Error; err != nil {
		return spec.NewError(0, "save station: %v", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeleteInternetRadioStation(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if !user.IsAdmin {
		return spec.NewError(50, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)

	stationID, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "no id provided: %v", err)
	}

	var station db.InternetRadioStation
	if err := c.dbc.Where("id=?", stationID.Value).First(&station).Error; err != nil {
		return spec.NewError(70, "id not found: %v", err)
	}

	if err := c.dbc.Delete(&station).Error; err != nil {
		return spec.NewError(70, "id not found: %v", err)
	}

	return spec.NewResponse()
}
