package ctrlsubsonic

import (
	"net/http"

	"github.com/mmcdole/gofeed"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/db"
)

func (c *Controller) ServeGetPodcasts(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	isIncludeEpisodes := params.GetOrBool("includeEpisodes", true)
	id, _ := params.GetID("id")
	podcasts, err := c.Podcasts.GetPodcastOrAll(id.Value, isIncludeEpisodes)
	if err != nil {
		return spec.NewError(10, "failed get podcast(s): %s", err)
	}
	sub := spec.NewResponse()
	sub.Podcasts = &spec.Podcasts{}
	for _, podcast := range podcasts {
		channel := spec.NewPodcastChannel(podcast)
		sub.Podcasts.List = append(sub.Podcasts.List, channel)
	}
	return sub
}

func (c *Controller) ServeGetNewestPodcasts(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	count := params.GetOrInt("count", 10)
	episodes, err := c.Podcasts.GetNewestPodcastEpisodes(count)
	if err != nil {
		return spec.NewError(10, "failed get podcast(s): %s", err)
	}
	sub := spec.NewResponse()
	sub.NewestPodcasts = &spec.NewestPodcasts{}
	for _, episode := range episodes {
		sub.NewestPodcasts.List = append(sub.NewestPodcasts.List, spec.NewPodcastEpisode(episode))
	}
	return sub
}

func (c *Controller) ServeDownloadPodcastEpisode(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.PodcastEpisode {
		return spec.NewError(10, "please provide a valid podcast episode id")
	}
	if err := c.Podcasts.DownloadEpisode(id.Value); err != nil {
		return spec.NewError(10, "failed to download episode: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeCreatePodcastChannel(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)
	rssURL, _ := params.Get("url")
	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(rssURL)
	if err != nil {
		return spec.NewError(10, "failed to parse feed: %s", err)
	}
	if _, err = c.Podcasts.AddNewPodcast(rssURL, feed); err != nil {
		return spec.NewError(10, "failed to add feed: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeRefreshPodcasts(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	if err := c.Podcasts.RefreshPodcasts(); err != nil {
		return spec.NewError(10, "failed to refresh feeds: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePodcastChannel(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.Podcast {
		return spec.NewError(10, "please provide a valid podcast id")
	}
	if err := c.Podcasts.DeletePodcast(id.Value); err != nil {
		return spec.NewError(10, "failed to delete podcast: %s", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePodcastEpisode(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	if (!user.IsAdmin) {
		return spec.NewError(10, "user not admin")
	}
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.PodcastEpisode {
		return spec.NewError(10, "please provide a valid podcast episode id")
	}
	if err := c.Podcasts.DeletePodcastEpisode(id.Value); err != nil {
		return spec.NewError(10, "failed to delete podcast: %s", err)
	}
	return spec.NewResponse()
}
