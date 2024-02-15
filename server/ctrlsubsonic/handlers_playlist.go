package ctrlsubsonic

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	playlistp "go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
)

func (c *Controller) ServeGetPlaylists(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
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
		rendered, err := playlistRender(c, params, playlistID, playlist, false)
		if err != nil {
			return spec.NewError(0, "error rendering playlist %q: %v", path, err)
		}
		sub.Playlists.List = append(sub.Playlists.List, rendered)
	}
	return sub
}

func (c *Controller) ServeGetPlaylist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	playlistID, err := params.GetFirst("id", "playlistId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	playlist, err := c.playlistStore.Read(playlistIDDecode(playlistID))
	if err != nil {
		return spec.NewError(70, "playlist with id %s not found", playlistID)
	}
	sub := spec.NewResponse()
	rendered, err := playlistRender(c, params, playlistID, playlist, true)
	if err != nil {
		return spec.NewError(0, "error rendering playlist: %v", err)
	}
	sub.Playlist = rendered
	return sub
}

func (c *Controller) ServeCreateOrUpdatePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	playlistID := params.GetFirstOr( /* default */ "", "id", "playlistId")
	playlistPath := playlistIDDecode(playlistID)

	var playlist playlistp.Playlist
	if pl, _ := c.playlistStore.Read(playlistPath); pl != nil {
		playlist = *pl
	}

	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewError(50, "you aren't allowed update that user's playlist")
	}

	playlist.UserID = user.ID
	playlist.UpdatedAt = time.Now()

	if val, err := params.Get("name"); err == nil {
		playlist.Name = val
	}

	playlist.Items = nil
	ids := params.GetOrIDList("songId", nil)
	for _, id := range ids {
		r, err := specidpaths.Locate(c.dbc, id)
		if err != nil {
			return spec.NewError(0, "lookup id %v: %v", id, err)
		}
		playlist.Items = append(playlist.Items, r.AbsPath())
	}

	if playlistPath == "" {
		playlistPath = playlistp.NewPath(user.ID, fmt.Sprint(time.Now().UnixMilli()))
		playlistID = playlistIDEncode(playlistPath)
	}

	if err := c.playlistStore.Write(playlistPath, &playlist); err != nil {
		return spec.NewError(0, "save playlist: %v", err)
	}

	sub := spec.NewResponse()
	rendered, err := playlistRender(c, params, playlistID, &playlist, true)
	if err != nil {
		return spec.NewError(0, "error rendering playlist: %v", err)
	}
	sub.Playlist = rendered
	return sub
}

func (c *Controller) ServeUpdatePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	playlistID := params.GetFirstOr( /* default */ "", "id", "playlistId")
	playlistPath := playlistIDDecode(playlistID)
	playlist, err := c.playlistStore.Read(playlistPath)
	if err != nil {
		return spec.NewError(0, "find playlist: %v", err)
	}

	// update meta info
	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewResponse()
	}

	if val, err := params.Get("name"); err == nil {
		playlist.Name = val
	}
	if val, err := params.Get("comment"); err == nil {
		playlist.Comment = val
	}
	if val, err := params.GetBool("public"); err == nil {
		playlist.IsPublic = val
	}

	// delete items
	if indexes, err := params.GetIntList("songIndexToRemove"); err == nil {
		sort.Sort(sort.Reverse(sort.IntSlice(indexes)))
		for _, i := range indexes {
			playlist.Items = append(playlist.Items[:i], playlist.Items[i+1:]...)
		}
	}

	// add items
	if ids, err := params.GetIDList("songIdToAdd"); err == nil {
		for _, id := range ids {
			item, err := specidpaths.Locate(c.dbc, id)
			if err != nil {
				return spec.NewError(0, "locate id %q: %v", id, err)
			}
			playlist.Items = append(playlist.Items, item.AbsPath())
		}
	}

	if err := c.playlistStore.Write(playlistPath, playlist); err != nil {
		return spec.NewError(0, "save playlist: %v", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePlaylist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	playlistID := params.GetFirstOr( /* default */ "", "id", "playlistId")
	if err := c.playlistStore.Delete(playlistIDDecode(playlistID)); err != nil {
		return spec.NewError(0, "delete playlist: %v", err)
	}
	return spec.NewResponse()
}

func playlistIDEncode(path string) string {
	return base64.URLEncoding.EncodeToString([]byte(path))
}

func playlistIDDecode(id string) string {
	path, _ := base64.URLEncoding.DecodeString(id)
	return string(path)
}

func playlistRender(c *Controller, params params.Params, playlistID string, playlist *playlistp.Playlist, withItems bool) (*spec.Playlist, error) {
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

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, path := range playlist.Items {
		file, err := specidpaths.Lookup(c.dbc, MusicPaths(c.musicPaths), c.podcastsPath, path)
		if err != nil {
			log.Printf("error looking up path %q: %s", path, err)
			continue
		}

		var trch *spec.TrackChild
		switch id := file.SID(); id.Type {
		case specid.Track:
			var track db.Track
			if err := c.dbc.Where("id=?", id.Value).Preload("Album").Preload("Album.Artists").Preload("Artists").Preload("TrackStar", "user_id=?", user.ID).Preload("TrackRating", "user_id=?", user.ID).Find(&track).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("load track by id: %w", err)
			}
			trch = spec.NewTCTrackByFolder(&track, track.Album)
			resp.Duration += track.Length
		case specid.PodcastEpisode:
			var pe db.PodcastEpisode
			if err := c.dbc.Preload("Podcast").Where("id=?", id.Value).Find(&pe).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("load podcast episode by id: %w", err)
			}
			trch = spec.NewTCPodcastEpisode(&pe)
			resp.Duration += pe.Length
		default:
			continue
		}
		trch.TranscodeMeta = transcodeMeta
		resp.List = append(resp.List, trch)
	}

	resp.SongCount = len(resp.List)

	return resp, nil
}
