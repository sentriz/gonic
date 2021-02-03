package ctrlsubsonic

import (
	"log"
	"net/http"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/scanner"
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
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	if id.Type == specid.Podcast || id.Type == specid.PodcastEpisode {
		return spec.NewError(10, "please provide a valid track id")
	}
	// fetch user to get lastfm session
	user := r.Context().Value(CtxUser).(*db.User)
	// fetch track for getting info to send to last.fm function
	track := &db.Track{}
	c.DB.
		Preload("Album").
		Preload("Artist").
		First(track, id.Value)
	// clients will provide time in miliseconds, so use that or
	// instead convert UnixNano to miliseconds
	optStampMili := params.GetOrInt("time", int(time.Now().UnixNano()/1e6))
	optSubmission := params.GetOrBool("submission", true)
	scrobbleErrs := []error{}
	for _, scrobbler := range c.Scrobblers {
		if err := scrobbler.Scrobble(user, track, optStampMili, optSubmission); err != nil {
			scrobbleErrs = append(scrobbleErrs, err)
		}
	}
	if len(scrobbleErrs) != 0 {
		return spec.NewError(0, "error when submitting: %v", scrobbleErrs)
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
		if err := c.Scanner.Start(scanner.ScanOptions{}); err != nil {
			log.Printf("error while scanning: %v\n", err)
		}
	}()
	return c.ServeGetScanStatus(r)
}

func (c *Controller) ServeGetScanStatus(r *http.Request) *spec.Response {
	var trackCount int
	c.DB.
		Model(db.Track{}).
		Count(&trackCount)
	sub := spec.NewResponse()
	sub.ScanStatus = &spec.ScanStatus{
		Scanning: scanner.IsScanning(),
		Count:    trackCount,
	}
	return sub
}

func (c *Controller) ServeGetUser(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	hasLastFM := user.LastFMSession != ""
	hasListenBrainz := user.ListenBrainzToken != ""
	sub := spec.NewResponse()
	sub.User = &spec.User{
		Username:          user.Name,
		AdminRole:         user.IsAdmin,
		JukeboxRole:       true,
		ScrobblingEnabled: hasLastFM || hasListenBrainz,
		PodcastRole:       c.Podcasts.PodcastBasePath != "",
		Folder:            []int{1},
	}
	return sub
}

func (c *Controller) ServeNotFound(r *http.Request) *spec.Response {
	return spec.NewError(70, "view not found")
}

func (c *Controller) ServeGetPlayQueue(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	queue := db.PlayQueue{}
	err := c.DB.
		Where("user_id=?", user.ID).
		Find(&queue).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewResponse()
	}
	sub := spec.NewResponse()
	sub.PlayQueue = &spec.PlayQueue{}
	sub.PlayQueue.Username = user.Name
	sub.PlayQueue.Position = queue.Position
	sub.PlayQueue.Current = queue.CurrentSID()
	sub.PlayQueue.Changed = queue.UpdatedAt
	sub.PlayQueue.ChangedBy = queue.ChangedBy
	trackIDs := queue.GetItems()
	sub.PlayQueue.List = make([]*spec.TrackChild, len(trackIDs))
	for i, id := range trackIDs {
		track := db.Track{}
		c.DB.
			Where("id=?", id).
			Preload("Album").
			Find(&track)
		sub.PlayQueue.List[i] = spec.NewTCTrackByFolder(&track, track.Album)
	}
	return sub
}

func (c *Controller) ServeSavePlayQueue(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	tracks, err := params.GetIDList("id")
	if err != nil {
		return spec.NewError(10, "please provide some `id` parameters")
	}
	trackIDs := make([]int, 0, len(tracks))
	for _, id := range tracks {
		trackIDs = append(trackIDs, id.Value)
	}
	user := r.Context().Value(CtxUser).(*db.User)
	queue := &db.PlayQueue{UserID: user.ID}
	c.DB.Where(queue).First(queue)
	queue.Current = params.GetOrID("current", specid.ID{}).Value
	queue.Position = params.GetOrInt("position", 0)
	queue.ChangedBy = params.GetOr("c", "") // must exist, middleware checks
	queue.SetItems(trackIDs)
	c.DB.Save(queue)
	return spec.NewResponse()
}

func (c *Controller) ServeGetSong(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "provide an `id` parameter")
	}
	track := &db.Track{}
	err = c.DB.
		Where("id=?", id.Value).
		Preload("Album").
		First(track).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(10, "couldn't find a track with that id")
	}
	sub := spec.NewResponse()
	sub.Track = spec.NewTrackByTags(track, track.Album)
	return sub
}

func (c *Controller) ServeGetRandomSongs(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	var tracks []*db.Track
	q := c.DB.DB.
		Limit(params.GetOrInt("size", 10)).
		Preload("Album").
		Joins("JOIN albums ON tracks.album_id=albums.id").
		Order(gorm.Expr("random()"))
	if year, err := params.GetInt("fromYear"); err == nil {
		q = q.Where("albums.tag_year >= ?", year)
	}
	if year, err := params.GetInt("toYear"); err == nil {
		q = q.Where("albums.tag_year <= ?", year)
	}
	if genre, err := params.Get("genre"); err == nil {
		q = q.Joins("JOIN track_genres ON track_genres.track_id=tracks.id")
		q = q.Joins("JOIN genres ON genres.id=track_genres.genre_id AND genres.name=?", genre)
	}
	q.Find(&tracks)
	sub := spec.NewResponse()
	sub.RandomTracks = &spec.RandomTracks{}
	sub.RandomTracks.List = make([]*spec.TrackChild, len(tracks))
	for i, track := range tracks {
		sub.RandomTracks.List[i] = spec.NewTrackByTags(track, track.Album)
	}
	return sub
}

func (c *Controller) ServeJukebox(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	getTracks := func() []*db.Track {
		var tracks []*db.Track
		ids, err := params.GetIDList("id")
		if err != nil {
			return tracks
		}
		for _, id := range ids {
			track := &db.Track{}
			c.DB.Preload("Album").First(track, id.Value)
			if track.ID != 0 {
				tracks = append(tracks, track)
			}
		}
		return tracks
	}
	getStatus := func() spec.JukeboxStatus {
		status := c.Jukebox.GetStatus()
		return spec.JukeboxStatus{
			CurrentIndex: status.CurrentIndex,
			Playing:      status.Playing,
			Gain:         status.Gain,
			Position:     status.Position,
		}
	}
	getStatusTracks := func() []*spec.TrackChild {
		tracks := c.Jukebox.GetTracks()
		ret := make([]*spec.TrackChild, len(tracks))
		for i, track := range tracks {
			ret[i] = spec.NewTrackByTags(track, track.Album)
		}
		return ret
	}
	switch act, _ := params.Get("action"); act {
	case "set":
		c.Jukebox.SetTracks(getTracks())
	case "add":
		c.Jukebox.AddTracks(getTracks())
	case "clear":
		c.Jukebox.ClearTracks()
	case "remove":
		index, err := params.GetInt("index")
		if err != nil {
			return spec.NewError(10, "please provide an id for remove actions")
		}
		c.Jukebox.RemoveTrack(index)
	case "stop":
		c.Jukebox.Stop()
	case "start":
		c.Jukebox.Start()
	case "skip":
		index, err := params.GetInt("index")
		if err != nil {
			return spec.NewError(10, "please provide an index for skip actions")
		}
		offset, _ := params.GetInt("offset")
		c.Jukebox.Skip(index, offset)
	case "get":
		sub := spec.NewResponse()
		sub.JukeboxPlaylist = &spec.JukeboxPlaylist{
			JukeboxStatus: getStatus(),
			List:          getStatusTracks(),
		}
		return sub
	}
	// all actions except get are expected to return a status
	sub := spec.NewResponse()
	status := getStatus()
	sub.JukeboxStatus = &status
	return sub
}
