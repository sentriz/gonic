package ctrlsubsonic

import (
	"net/http"

	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

func (c *Controller) ServeGetPodcasts(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Podcasts = &spec.Podcasts{
		List: []struct{}{},
	}
	return sub
}
