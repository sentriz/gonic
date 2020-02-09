package ctrlsubsonic

import (
	"net/http"

	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
)

// NOTE: when these are implemented, they should be moved to their
// respective _by_folder or _by_tag file

func (c *Controller) ServeGetArtistInfo(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.ArtistInfo = &spec.ArtistInfo{}
	return sub
}

func (c *Controller) ServeGetArtistInfoTwo(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.ArtistInfoTwo = &spec.ArtistInfo{}
	return sub
}

func (c *Controller) ServeGetGenres(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Genres = &spec.Genres{}
	return sub
}
