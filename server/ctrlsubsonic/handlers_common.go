package ctrlsubsonic

import (
	"log"
	"net/http"
	"strconv"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/scanner"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
	"senan.xyz/g/gonic/server/key"
	"senan.xyz/g/gonic/server/lastfm"
	"senan.xyz/g/gonic/server/parsing"
)

func lowerUDecOrHash(in string) string {
	lower := unicode.ToLower(rune(in[0]))
	if !unicode.IsLetter(lower) {
		return "#"
	}
	return string(lower)
}

func (c *Controller) ServeGetLicence(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Licence = &spec.Licence{
		Valid: true,
	}
	return sub
}

func (c *Controller) ServePing(r *http.Request) *spec.Response {
	return spec.NewResponse()
}

func (c *Controller) ServeScrobble(r *http.Request) *spec.Response {
	id, err := parsing.GetIntParam(r, "id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	// fetch user to get lastfm session
	user := r.Context().Value(key.User).(*model.User)
	if user.LastFMSession == "" {
		return spec.NewError(0, "you don't have a last.fm session")
	}
	// fetch track for getting info to send to last.fm function
	track := &model.Track{}
	c.DB.
		Preload("Album").
		Preload("Artist").
		First(track, id)
	// scrobble with above info
	err = lastfm.Scrobble(
		c.DB.GetSetting("lastfm_api_key"),
		c.DB.GetSetting("lastfm_secret"),
		user.LastFMSession,
		track,
		// clients will provide time in miliseconds, so use that or
		// instead convert UnixNano to miliseconds
		parsing.GetIntParamOr(r, "time", int(time.Now().UnixNano()/1e6)),
		parsing.GetStrParamOr(r, "submission", "true") != "false",
	)
	if err != nil {
		return spec.NewError(0, "error when submitting: %v", err)
	}
	return spec.NewResponse()
}

func (c *Controller) ServeGetMusicFolders(r *http.Request) *spec.Response {
	folders := &spec.MusicFolders{}
	folders.List = []*spec.MusicFolder{
		{ID: 1, Name: "music"},
	}
	sub := spec.NewResponse()
	sub.MusicFolders = folders
	return sub
}

func (c *Controller) ServeStartScan(r *http.Request) *spec.Response {
	go func() {
		if err := c.Scanner.Start(); err != nil {
			log.Printf("error while scanning: %v\n", err)
		}
	}()
	return c.ServeGetScanStatus(r)
}

func (c *Controller) ServeGetScanStatus(r *http.Request) *spec.Response {
	var trackCount int
	c.DB.
		Model(model.Track{}).
		Count(&trackCount)
	sub := spec.NewResponse()
	sub.ScanStatus = &spec.ScanStatus{
		Scanning: scanner.IsScanning(),
		Count:    trackCount,
	}
	return sub
}

func (c *Controller) ServeGetUser(r *http.Request) *spec.Response {
	user := r.Context().Value(key.User).(*model.User)
	sub := spec.NewResponse()
	sub.User = &spec.User{
		Username:          user.Name,
		AdminRole:         user.IsAdmin,
		ScrobblingEnabled: user.LastFMSession != "",
		Folder:            []int{1},
	}
	return sub
}

func (c *Controller) ServeNotFound(r *http.Request) *spec.Response {
	return spec.NewError(70, "view not found")
}

func (c *Controller) ServeGetPlaylists(r *http.Request) *spec.Response {
	user := r.Context().Value(key.User).(*model.User)
	var playlists []*model.Playlist
	c.DB.
		Where("user_id = ?", user.ID).
		Find(&playlists)
	sub := spec.NewResponse()
	sub.Playlists = &spec.Playlists{
		List: make([]*spec.Playlist, len(playlists)),
	}
	for i, playlist := range playlists {
		sub.Playlists.List[i] = spec.NewPlaylist(playlist)
		sub.Playlists.List[i].Owner = user.Name
	}
	return sub
}

func (c *Controller) ServeGetPlaylist(r *http.Request) *spec.Response {
	playlistID, err := parsing.GetIntParam(r, "id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	playlist := model.Playlist{}
	err = c.DB.
		Where("id = ?", playlistID).
		Find(&playlist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(70, "playlist with id `%d` not found", playlistID)
	}
	var tracks []*model.Track
	c.DB.
		Joins(`
            JOIN playlist_items
		    ON playlist_items.track_id = tracks.id
		`).
		Where("playlist_items.playlist_id = ?", playlistID).
		Group("tracks.id").
		Order("playlist_items.created_at").
		Preload("Album").
		Find(&tracks)
	user := r.Context().Value(key.User).(*model.User)
	sub := spec.NewResponse()
	sub.Playlist = spec.NewPlaylist(&playlist)
	sub.Playlist.Owner = user.Name
	sub.Playlist.List = make([]*spec.TrackChild, len(tracks))
	for i, track := range tracks {
		sub.Playlist.List[i] = spec.NewTCTrackByFolder(track, track.Album)
	}
	return sub
}

func (c *Controller) ServeUpdatePlaylist(r *http.Request) *spec.Response {
	playlistID, _ := parsing.GetFirstIntParamOf(r, "id", "playlistId")
	//
	// begin updating meta
	playlist := &model.Playlist{}
	c.DB.
		Where("id = ?", playlistID).
		First(playlist)
	user := r.Context().Value(key.User).(*model.User)
	playlist.UserID = user.ID
	if name := parsing.GetStrParam(r, "name"); name != "" {
		playlist.Name = name
	}
	if comment := parsing.GetStrParam(r, "comment"); comment != "" {
		playlist.Comment = comment
	}
	c.DB.Save(playlist)
	//
	// begin delete tracks
	if indexes, ok := r.URL.Query()["songIndexToRemove"]; ok {
		trackIDs := []int{}
		c.DB.
			Order("created_at").
			Model(&model.PlaylistItem{}).
			Where("playlist_id = ?", playlistID).
			Pluck("track_id", &trackIDs)
		for _, indexStr := range indexes {
			i, err := strconv.Atoi(indexStr)
			if err != nil {
				continue
			}
			c.DB.Delete(&model.PlaylistItem{},
				"track_id = ?", trackIDs[i])
		}
	}
	//
	// begin add tracks
	if toAdd := parsing.GetFirstParamOf(r, "songId", "songIdToAdd"); toAdd != nil {
		for _, trackIDStr := range toAdd {
			trackID, err := strconv.Atoi(trackIDStr)
			if err != nil {
				continue
			}
			c.DB.Save(&model.PlaylistItem{
				PlaylistID: playlist.ID,
				TrackID:    trackID,
			})
		}
	}
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePlaylist(r *http.Request) *spec.Response {
	c.DB.
		Where("id = ?", parsing.GetIntParamOr(r, "id", 0)).
		Delete(&model.Playlist{})
	return spec.NewResponse()
}
