package ctrlsubsonic

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/scanner"
	"senan.xyz/g/gonic/server/ctrlsubsonic/params"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
	"senan.xyz/g/gonic/server/lastfm"
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
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	// fetch user to get lastfm session
	user := r.Context().Value(CtxUser).(*model.User)
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
		params.GetIntOr("time", int(time.Now().UnixNano()/1e6)),
		params.GetOr("submission", "true") != "false",
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
	user := r.Context().Value(CtxUser).(*model.User)
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
	user := r.Context().Value(CtxUser).(*model.User)
	var playlists []*model.Playlist
	c.DB.Where("user_id = ?", user.ID).Find(&playlists)
	sub := spec.NewResponse()
	sub.Playlists = &spec.Playlists{
		List: make([]*spec.Playlist, len(playlists)),
	}
	for i, playlist := range playlists {
		sub.Playlists.List[i] = spec.NewPlaylist(playlist)
		sub.Playlists.List[i].Owner = user.Name
		sub.Playlists.List[i].SongCount = playlist.TrackCount
	}
	return sub
}

func (c *Controller) ServeGetPlaylist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	playlistID, err := params.GetInt("id")
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
	user := r.Context().Value(CtxUser).(*model.User)
	sub := spec.NewResponse()
	sub.Playlist = spec.NewPlaylist(&playlist)
	sub.Playlist.Owner = user.Name
	sub.Playlist.SongCount = playlist.TrackCount
	trackIDs := playlist.GetItems()
	sub.Playlist.List = make([]*spec.TrackChild, len(trackIDs))
	for i, id := range trackIDs {
		track := model.Track{}
		c.DB.
			Where("id = ?", id).
			Preload("Album").
			Find(&track)
		sub.Playlist.List[i] = spec.NewTCTrackByFolder(&track, track.Album)
	}
	return sub
}

func (c *Controller) ServeUpdatePlaylist(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*model.User)
	params := r.Context().Value(CtxParams).(params.Params)
	var playlistID int
	if p := params.GetFirstList("id", "playlistId"); p != nil {
		playlistID, _ = strconv.Atoi(p[0])
	}
	// playlistID may be 0 from above. in that case we get a new playlist
	// as intended
	playlist := &model.Playlist{ID: playlistID}
	c.DB.Where(playlist).First(playlist)
	// ** begin update meta info
	playlist.UserID = user.ID
	if val := params.Get("name"); val != "" {
		playlist.Name = val
	}
	if val := params.Get("comment"); val != "" {
		playlist.Comment = val
	}
	trackIDs := playlist.GetItems()
	// ** begin delete items
	if p := params.GetFirstListInt("songIndexToRemove"); p != nil {
		sort.Sort(sort.Reverse(sort.IntSlice(trackIDs)))
		for _, i := range p {
			trackIDs = append(trackIDs[:i], trackIDs[i+1:]...)
		}
	}
	// ** begin add items
	if p := params.GetFirstListInt("songId", "songIdToAdd"); p != nil {
		trackIDs = append(trackIDs, p...)
	}
	//
	playlist.SetItems(trackIDs)
	c.DB.Save(playlist)
	return spec.NewResponse()
}

func (c *Controller) ServeDeletePlaylist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	c.DB.
		Where("id = ?", params.GetIntOr("id", 0)).
		Delete(&model.Playlist{})
	return spec.NewResponse()
}
