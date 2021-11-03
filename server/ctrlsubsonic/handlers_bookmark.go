package ctrlsubsonic

import (
	"errors"
	"net/http"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/db"
)

func (c *Controller) ServeGetBookmarks(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	bookmarks := []*db.Bookmark{}
	err := c.DB.
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
		specid := &specid.ID{
			Type:  specid.IDT(bookmark.EntryIDType),
			Value: bookmark.EntryID,
		}
		entries := []*spec.BookmarkEntry{{
			ID:   specid,
			Type: bookmark.EntryIDType,
		}}
		sub.Bookmarks.List = append(sub.Bookmarks.List, &spec.Bookmark{
			Username: user.Name,
			Position: bookmark.Position,
			Comment:  bookmark.Comment,
			Created:  bookmark.CreatedAt,
			Changed:  bookmark.UpdatedAt,
			Entries:  entries,
		})
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
	c.DB.FirstOrCreate(bookmark, db.Bookmark{
		UserID:      user.ID,
		EntryIDType: string(id.Type),
		EntryID:     id.Value,
	})
	bookmark.Comment = params.GetOr("comment", "")
	bookmark.Position = params.GetOrInt("position", 0)
	c.DB.Save(bookmark)
	return spec.NewResponse()
}

func (c *Controller) ServeDeleteBookmark(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	c.DB.
		Where("user_id=? AND entry_id_type=? AND entry_id=?", user.ID, id.Type, id.Value).
		Delete(&db.Bookmark{})
	return spec.NewResponse()
}
