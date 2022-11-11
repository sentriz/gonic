package ctrlsubsonic

import (
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func playlistRender(c *Controller, playlist *db.Playlist, params params.Params) *spec.Playlist {
	user := &db.User{}
	c.DB.Where("id=?", playlist.UserID).Find(user)

	resp := &spec.Playlist{
		ID:        playlist.ID,
		Name:      playlist.Name,
		Comment:   playlist.Comment,
		Created:   playlist.CreatedAt,
		SongCount: playlist.TrackCount,
		Public:    playlist.IsPublic,
		Owner:     user.Name,
	}

	trackIDs := playlist.GetItems()
	resp.List = make([]*spec.TrackChild, len(trackIDs))

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, id := range trackIDs {
		switch id.Type {
		case specid.Track:
			track := db.Track{}
			err := c.DB.
				Where("id=?", id.Value).
				Preload("Album").
				Preload("Album.TagArtist").
				Preload("TrackStar", "user_id=?", user.ID).
				Preload("TrackRating", "user_id=?", user.ID).
				Find(&track).
				Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("wasn't able to find track with id %d", id.Value)
				continue
			}
			resp.List[i] = spec.NewTCTrackByFolder(&track, track.Album)
			resp.Duration += track.Length
		case specid.PodcastEpisode:
			pe := db.PodcastEpisode{}
			err := c.DB.
				Where("id=?", id.Value).
				Find(&pe).
				Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("wasn't able to find podcast episode with id %d", id.Value)
				continue
			}
			p := db.Podcast{}
			err = c.DB.
				Where("id=?", pe.PodcastID).
				Find(&p).
				Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("wasn't able to find podcast with id %d", pe.PodcastID)
				continue
			}
			resp.List[i] = spec.NewTCPodcastEpisode(&pe, &p)
			resp.Duration += pe.Length
		}
		resp.List[i].TranscodedContentType = transcodeMIME
		resp.List[i].TranscodedSuffix = transcodeSuffix
	}
	return resp
}

func (c *Controller) ServeGetPlaylists(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var playlists []*db.Playlist
	c.DB.Where("user_id=?", user.ID).Or("is_public=?", true).Find(&playlists)
	sub := spec.NewResponse()
	sub.Playlists = &spec.Playlists{
		List: make([]*spec.Playlist, len(playlists)),
	}
	for i, playlist := range playlists {
		sub.Playlists.List[i] = playlistRender(c, playlist, params)
	}
	return sub
}

func (c *Controller) ServeGetPlaylist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	playlistID, err := params.GetFirstInt("id", "playlistId")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	playlist := db.Playlist{}
	err = c.DB.
		Where("id=?", playlistID).
		Find(&playlist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "playlist with id `%d` not found", playlistID)
	}
	sub := spec.NewResponse()
	sub.Playlist = playlistRender(c, &playlist, params)
	return sub
}

func (c *Controller) ServeCreatePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)
	playlistID := params.GetFirstOrInt( /* default */ 0, "id", "playlistId")
	// playlistID may be 0 from above. in that case we get a new playlist
	// as intended
	var playlist db.Playlist
	c.DB.
		Where("id=?", playlistID).
		FirstOrCreate(&playlist)

		// update meta info
	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewResponse()
	}
	playlist.UserID = user.ID
	if val, err := params.Get("name"); err == nil {
		playlist.Name = val
	}

	// replace song IDs
	trackIDs, _ := params.GetIDList("songId")
	// Set the items of the playlist
	playlist.SetItems(trackIDs)
	c.DB.Save(playlist)

	sub := spec.NewResponse()
	sub.Playlist = playlistRender(c, &playlist, params)
	return sub
}

func (c *Controller) ServeUpdatePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)
	playlistID := params.GetFirstOrInt( /* default */ 0, "id", "playlistId")
	// playlistID may be 0 from above. in that case we get a new playlist
	// as intended
	var playlist db.Playlist
	c.DB.
		Where("id=?", playlistID).
		FirstOrCreate(&playlist)

		// update meta info
	if playlist.UserID != 0 && playlist.UserID != user.ID {
		return spec.NewResponse()
	}
	playlist.UserID = user.ID
	if val, err := params.Get("name"); err == nil {
		playlist.Name = val
	}
	if val, err := params.Get("comment"); err == nil {
		playlist.Comment = val
	}
	if val, err := params.GetBool("public"); err == nil {
		playlist.IsPublic = val
	}
	trackIDs := playlist.GetItems()

	// delete items
	if p, err := params.GetIntList("songIndexToRemove"); err == nil {
		sort.Sort(sort.Reverse(sort.IntSlice(p)))
		for _, i := range p {
			trackIDs = append(trackIDs[:i], trackIDs[i+1:]...)
		}
	}

	// add items
	if p, err := params.GetIDList("songIdToAdd"); err == nil {
		trackIDs = append(trackIDs, p...)
	}

	playlist.SetItems(trackIDs)
	c.DB.Save(playlist)
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePlaylist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	c.DB.
		Where("id=?", params.GetOrInt("id", 0)).
		Delete(&db.Playlist{})
	return spec.NewResponse()
}
