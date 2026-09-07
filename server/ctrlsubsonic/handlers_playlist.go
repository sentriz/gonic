package ctrlsubsonic

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"go.senan.xyz/gonic/db"
	playlistp "go.senan.xyz/gonic/playlist"
	paramsp "go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
)

func (c *Controller) ServeGetPlaylists(r *http.Request) *spec.Response {
	rnd := render(c, r)
	user := r.Context().Value(CtxUser).(*db.User)
	paths, err := c.playlistStore.List()
	if err != nil {
		return spec.NewError(0, "error listing playlists: %v", err)
	}
	sub := spec.NewResponse()
	sub.Playlists = &spec.Playlists{
		List: []*spec.Playlist{},
	}
	for _, path := range paths {
		playlist, err := c.playlistStore.Read(path)
		if err != nil {
			return spec.NewError(0, "error reading playlist %q: %v", path, err)
		}
		if playlist.UserID != user.ID && !playlist.IsPublic {
			continue
		}
		playlistID := playlistIDEncode(path)
		rendered, err := playlistRender(c, rnd, playlist, playlistID, false)
		if err != nil {
			return spec.NewError(0, "error rendering playlist %q: %v", path, err)
		}
		sub.Playlists.List = append(sub.Playlists.List, rendered)
	}
	return sub
}

func (c *Controller) ServeGetPlaylist(r *http.Request) *spec.Response {
	rnd := render(c, r)
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(paramsp.Params)
	playlistID, err := params.GetFirstID("id", "playlistId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	playlist, err := c.playlistStore.Read(playlistIDDecode(playlistID))
	if err != nil {
		return spec.NewError(70, "playlist with id %s not found", playlistID)
	}
	if playlist.UserID != user.ID && !playlist.IsPublic {
		return spec.NewError(50, "you aren't allowed to read that user's playlist")
	}
	sub := spec.NewResponse()
	rendered, err := playlistRender(c, rnd, playlist, playlistID, true)
	if err != nil {
		return spec.NewError(0, "error rendering playlist: %v", err)
	}
	sub.Playlist = rendered
	return sub
}

func (c *Controller) ServeCreateOrUpdatePlaylist(r *http.Request) *spec.Response {
	rnd := render(c, r)
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(paramsp.Params)

	playlistID, _ := params.GetFirstID("id", "playlistId")
	playlistPath := playlistIDDecode(playlistID)

	var playlist playlistp.Playlist
	if playlistPath != "" {
		if pl, err := c.playlistStore.Read(playlistPath); err == nil && pl != nil {
			playlist = *pl
		}
	}

	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewError(50, "you aren't allowed update that user's playlist")
	}

	// path not found, make sure we don't use caller's provided path since it might be in another user's dir
	if playlist.UserID == 0 {
		playlistPath = playlistp.NewPath(user.ID, fmt.Sprint(time.Now().UnixMilli()))
		playlistID = playlistIDEncode(playlistPath)
	}

	playlist.UserID = user.ID
	playlist.UpdatedAt = time.Now()

	if val, err := params.Get("name"); err == nil {
		playlist.Name = val
	}

	playlist.Items = nil
	ids, err := params.GetIDList("songId")
	if err != nil && !errors.Is(err, paramsp.ErrNoValues) {
		return spec.NewError(10, "please provide valid song ids: %v", err)
	}
	for _, id := range ids {
		r, err := specidpaths.Locate(c.dbc, id)
		if err != nil {
			return spec.NewError(70, "couldn't find a track with id %v: %v", id, err)
		}
		playlist.Items = append(playlist.Items, r.AbsPath())
	}

	if err := c.playlistStore.Write(playlistPath, &playlist); err != nil {
		return spec.NewError(0, "save playlist: %v", err)
	}

	sub := spec.NewResponse()
	rendered, err := playlistRender(c, rnd, &playlist, playlistID, true)
	if err != nil {
		return spec.NewError(0, "error rendering playlist: %v", err)
	}
	sub.Playlist = rendered
	return sub
}

func (c *Controller) ServeUpdatePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	p := r.Context().Value(CtxParams).(paramsp.Params)

	playlistID, err := p.GetFirstID("id", "playlistId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` or `playlistId` parameter")
	}
	playlistPath := playlistIDDecode(playlistID)
	playlist, err := c.playlistStore.Read(playlistPath)
	if err != nil {
		return spec.NewError(0, "find playlist: %v", err)
	}

	// update meta info
	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewResponse()
	}

	if val, err := p.Get("name"); err == nil {
		playlist.Name = val
	}
	if val, err := p.Get("comment"); err == nil {
		playlist.Comment = val
	}
	if val, err := p.GetBool("public"); err == nil {
		playlist.IsPublic = val
	}

	// delete items
	if indexes, err := p.GetIntList("songIndexToRemove"); err == nil {
		sort.Sort(sort.Reverse(sort.IntSlice(indexes)))
		for _, i := range indexes {
			playlist.Items = append(playlist.Items[:i], playlist.Items[i+1:]...)
		}
	}

	// add items
	ids, err := p.GetIDList("songIdToAdd")
	if err != nil && !errors.Is(err, paramsp.ErrNoValues) {
		return spec.NewError(10, "please provide valid song ids: %v", err)
	}
	for _, id := range ids {
		item, err := specidpaths.Locate(c.dbc, id)
		if err != nil {
			return spec.NewError(70, "couldn't find a track with id %v: %v", id, err)
		}
		playlist.Items = append(playlist.Items, item.AbsPath())
	}

	if err := c.playlistStore.Write(playlistPath, playlist); err != nil {
		return spec.NewError(0, "save playlist: %v", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(paramsp.Params)
	playlistID, err := params.GetFirstID("id", "playlistId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` or `playlistId` parameter")
	}
	playlistPath := playlistIDDecode(playlistID)
	playlist, err := c.playlistStore.Read(playlistPath)
	if err != nil {
		return spec.NewError(70, "playlist with id %s not found", playlistID)
	}
	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewError(50, "you aren't allowed to delete that user's playlist")
	}
	if err := c.playlistStore.Delete(playlistPath); err != nil {
		return spec.NewError(0, "delete playlist: %v", err)
	}
	return spec.NewResponse()
}

func playlistIDEncode(path string) specid.ID {
	return specid.ID{
		Type:        specid.Playlist,
		StringValue: base64.URLEncoding.EncodeToString([]byte(path)),
	}
}

func playlistIDDecode(id specid.ID) string {
	path, _ := base64.URLEncoding.DecodeString(id.StringValue)
	return string(path)
}

func playlistRender(c *Controller, rnd spec.Render, playlist *playlistp.Playlist, playlistID specid.ID, withItems bool) (*spec.Playlist, error) {
	user := &db.User{}
	if err := c.dbc.Where("id=?", playlist.UserID).Find(user).Error; err != nil {
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	resp := &spec.Playlist{
		ID:        playlistID,
		Name:      playlist.Name,
		Comment:   playlist.Comment,
		Created:   playlist.UpdatedAt,
		Changed:   playlist.UpdatedAt,
		SongCount: len(playlist.Items),
		Public:    playlist.IsPublic,
		Owner:     user.Name,
	}
	if !withItems {
		return resp, nil
	}

	ids := make([]specid.ID, 0, len(playlist.Items))
	for _, path := range playlist.Items {
		id, err := specidpaths.Lookup(c.dbc, MusicPaths(c.musicPaths), c.podcastsPath, path)
		if err != nil {
			log.Printf("error looking up path %q: %s", path, err)
			continue
		}
		ids = append(ids, *id)
	}

	entries, err := spec.EntriesByFolder(rnd, ids)
	if err != nil {
		return nil, fmt.Errorf("render playlist entries: %w", err)
	}
	for i, entry := range entries {
		if entry == nil {
			log.Printf("skipping missing playlist entry %s", ids[i])
			continue
		}
		resp.Duration += entry.Duration
		resp.List = append(resp.List, entry)
	}
	resp.SongCount = len(resp.List)
	return resp, nil
}
