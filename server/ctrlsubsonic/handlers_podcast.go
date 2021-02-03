package ctrlsubsonic

import (
	"net/http"

	"github.com/mmcdole/gofeed"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/db"
)

func (c *Controller) ServeGetPodcasts(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	isIncludeEpisodes := true
	if ie, err := params.GetBool("includeEpisodes"); !ie && err == nil {
		isIncludeEpisodes = false
	}
	sub := spec.NewResponse()
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		sub.Podcasts, err = c.Podcasts.GetAllPodcasts(user.ID, isIncludeEpisodes)
		if err != nil {
			return spec.NewError(10, "Failed to retrieve podcasts: %s", err)
		}
		return sub
	}
	sub.Podcasts, _ = c.Podcasts.GetPodcast(id.Value, user.ID, isIncludeEpisodes)
	return sub
}

func (c *Controller) ServeDownloadPodcastEpisode(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.PodcastEpisode {
		return spec.NewError(10, "Please provide a valid podcast episode id")
	}
	if err := c.Podcasts.DownloadEpisode(id.Value); err != nil {
		return spec.NewError(10, "Failed to download episode: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeCreatePodcastChannel(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)
	rssURL, _ := params.Get("url")
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rssURL)
	if err != nil {
		return spec.NewError(10, "Failed to parse feed: %s", err)
	}
	_, err = c.Podcasts.AddNewPodcast(feed, user.ID)
	if err != nil {
		return spec.NewError(10, "Failed to add feed: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeRefreshPodcasts(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	err := c.Podcasts.RefreshPodcasts(user.ID, false)
	if err != nil {
		return spec.NewError(10, "Failed to refresh feeds: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePodcastChannel(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.Podcast {
		return spec.NewError(10, "Please provide a valid podcast ID")
	}
	err = c.Podcasts.DeletePodcast(user.ID, id.Value)
	if err != nil {
		return spec.NewError(10, "Failed to delete podcast: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePodcastEpisode(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.PodcastEpisode {
		return spec.NewError(10, "Please provide a valid podcast episode ID")
	}
	err = c.Podcasts.DeletePodcastEpisode(id.Value)
	if err != nil {
		return spec.NewError(10, "Failed to delete podcast: %s", err)
	}
	return spec.NewResponse()
}
