package ctrlsubsonic

import (
	"net/http"

	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
)

// NOTE: when these are implemented, they should be moved to their
// respective _by_folder or _by_tag file

func (c *Controller) ServeGetGenres(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Genres = &spec.Genres{}
	sub.Genres.List = []*spec.Genre{}
	return sub
}
