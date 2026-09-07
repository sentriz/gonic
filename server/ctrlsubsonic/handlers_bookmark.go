package ctrlsubsonic

import (
	"net/http"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func (c *Controller) ServeGetBookmarks(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	rnd := render(c, r)

	bookmarks := []*db.Bookmark{}

	err := c.dbc.
		Where("user_id=?", user.ID).
		Find(&bookmarks).
		Error
	if err != nil {
		return spec.NewError(0, "error finding bookmarks: %v", err)
	}

	sub := spec.NewResponse()
	sub.Bookmarks = &spec.Bookmarks{
		List: []*spec.Bookmark{},
	}

	entryIDs := make([]specid.ID, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		entryIDs = append(entryIDs, specid.ID{Type: specid.IDT(bookmark.EntryIDType), Value: bookmark.EntryID})
	}
	entries, err := spec.EntriesByTags(rnd, entryIDs)
	if err != nil {
		return spec.NewError(0, "render bookmark entries: %v", err)
	}

	for i, bookmark := range bookmarks {
		// a bookmark can outlive its entry, eg. when files are moved. skip those
		// rather than failing the whole response.
		if entries[i] == nil {
			continue
		}
		sub.Bookmarks.List = append(sub.Bookmarks.List, &spec.Bookmark{
			Username: user.Name,
			Position: bookmark.Position,
			Comment:  bookmark.Comment,
			Created:  bookmark.CreatedAt,
			Changed:  bookmark.UpdatedAt,
			Entry:    entries[i],
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
	err = c.dbc.
		FirstOrCreate(bookmark, db.Bookmark{
			UserID:      user.ID,
			EntryIDType: db.BookmarkEntry(id.Type),
			EntryID:     id.Value,
		}).
		Error
	if err != nil {
		return spec.NewError(0, "error finding bookmark: %v", err)
	}
	bookmark.Comment = params.GetOr("comment", "")
	bookmark.Position = params.GetOrInt("position", 0)
	if err := c.dbc.Save(bookmark).Error; err != nil {
		return spec.NewError(0, "error saving bookmark: %v", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeleteBookmark(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	err = c.dbc.
		Where("user_id=? AND entry_id_type=? AND entry_id=?", user.ID, db.BookmarkEntry(id.Type), id.Value).
		Delete(&db.Bookmark{}).
		Error
	if err != nil {
		return spec.NewError(0, "error deleting bookmark: %v", err)
	}
	return spec.NewResponse()
}
