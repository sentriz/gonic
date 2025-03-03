package ctrlsubsonic

import (
	"errors"
	"net/http"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func (c *Controller) ServeGetBookmarks(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	bookmarks := []*db.Bookmark{}
	err := c.dbc.
		Where("user_id=?", user.ID).
		Find(&bookmarks).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewResponse()
	}

	sub := spec.NewResponse()
	sub.Bookmarks = &spec.Bookmarks{
		List: []*spec.Bookmark{},
	}

	for _, bookmark := range bookmarks {
		respBookmark := &spec.Bookmark{
			Username: user.Name,
			Position: bookmark.Position,
			Comment:  bookmark.Comment,
			Created:  bookmark.CreatedAt,
			Changed:  bookmark.UpdatedAt,
		}

		switch specid.IDT(bookmark.EntryIDType) {
		case specid.Track:
			var track db.Track
			err := c.dbc.
				Preload("Album").
				Find(&track, "id=?", bookmark.EntryID).
				Error
			if err != nil {
				return spec.NewError(10, "finding entry: %v", err)
			}
			respBookmark.Entry = spec.NewTrackByTags(&track, track.Album)
			break
		case specid.PodcastEpisode:
			var podcastEpisode db.PodcastEpisode
			err := c.dbc.
				Preload("Podcast").
				Find(&podcastEpisode, "id=?", bookmark.EntryID).
				Error
			if err != nil {
				return spec.NewError(10, "finding entry: %v", err)
			}
			respBookmark.Entry = spec.NewTCPodcastEpisode(&podcastEpisode)
			break
		default:
			continue
		}

		sub.Bookmarks.List = append(sub.Bookmarks.List, respBookmark)
	}

	return sub
}

func (c *Controller) ServeCreateBookmark(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	bookmark := &db.Bookmark{}
	c.dbc.FirstOrCreate(bookmark, db.Bookmark{
		UserID:      user.ID,
		EntryIDType: string(id.Type),
		EntryID:     id.Value,
	})
	bookmark.Comment = params.GetOr("comment", "")
	bookmark.Position = params.GetOrInt("position", 0)
	c.dbc.Save(bookmark)
	return spec.NewResponse()
}

func (c *Controller) ServeDeleteBookmark(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	c.dbc.
		Where("user_id=? AND entry_id_type=? AND entry_id=?", user.ID, id.Type, id.Value).
		Delete(&db.Bookmark{})
	return spec.NewResponse()
}
