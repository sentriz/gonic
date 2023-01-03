package ctrlsubsonic

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/multierr"
	"go.senan.xyz/gonic/paths"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func lowerUDecOrHash(in string) string {
	lower := unicode.ToLower(rune(in[0]))
	if !unicode.IsLetter(lower) {
		return "#"
	}
	return string(lower)
}

func getMusicFolder(musicPaths paths.MusicPaths, p params.Params) string {
	idx, err := p.GetInt("musicFolderId")
	if err != nil {
		return ""
	}
	if idx < 0 || idx >= len(musicPaths) {
		return ""
	}
	return musicPaths[idx].Path
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
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	id, err := params.GetID("id")
	if err != nil || id.Type != specid.Track {
		return spec.NewError(10, "please provide a track `id` track parameter")
	}

	track := &db.Track{}
	if err := c.DB.Preload("Album").Preload("Artist").First(track, id.Value).Error; err != nil {
		return spec.NewError(0, "error finding track: %v", err)
	}

	optStamp := params.GetOrTime("time", time.Now())
	optSubmission := params.GetOrBool("submission", true)

	if err := streamUpdateStats(c.DB, user.ID, track.Album.ID, optStamp); err != nil {
		return spec.NewError(0, "error updating stats: %v", err)
	}

	var scrobbleErrs multierr.Err
	for _, scrobbler := range c.Scrobblers {
		if err := scrobbler.Scrobble(user, track, optStamp, optSubmission); err != nil {
			scrobbleErrs.Add(err)
		}
	}
	if scrobbleErrs.Len() > 0 {
		return spec.NewError(0, "error when submitting: %s", scrobbleErrs.Error())
	}

	return spec.NewResponse()
}

func (c *Controller) ServeGetMusicFolders(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.MusicFolders = &spec.MusicFolders{}
	sub.MusicFolders.List = make([]*spec.MusicFolder, len(c.MusicPaths))
	for i, path := range c.MusicPaths {
		sub.MusicFolders.List[i] = &spec.MusicFolder{ID: i, Name: path.DisplayAlias()}
	}
	return sub
}

func (c *Controller) ServeStartScan(r *http.Request) *spec.Response {
	go func() {
		if _, err := c.Scanner.ScanAndClean(scanner.ScanOptions{}); err != nil {
			log.Printf("error while scanning: %v\n", err)
		}
	}()
	return c.ServeGetScanStatus(r)
}

func (c *Controller) ServeGetScanStatus(r *http.Request) *spec.Response {
	var trackCount int
	if err := c.DB.Model(db.Track{}).Count(&trackCount).Error; err != nil {
		return spec.NewError(0, "error finding track count: %v", err)
	}

	sub := spec.NewResponse()
	sub.ScanStatus = &spec.ScanStatus{
		Scanning: c.Scanner.IsScanning(),
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
		JukeboxRole:       c.Jukebox != nil,
		PodcastRole:       c.Podcasts != nil,
		DownloadRole:      true,
		ScrobblingEnabled: hasLastFM || hasListenBrainz,
		Folder:            []int{1},
	}
	return sub
}

func (c *Controller) ServeNotFound(r *http.Request) *spec.Response {
	return spec.NewError(70, "view not found")
}

func (c *Controller) ServeGetPlayQueue(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var queue db.PlayQueue
	err := c.DB.
		Where("user_id=?", user.ID).
		Find(&queue).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
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

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, id := range trackIDs {
		switch id.Type {
		case specid.Track:
			track := db.Track{}
			c.DB.
				Where("id=?", id.Value).
				Preload("Album").
				Preload("TrackStar", "user_id=?", user.ID).
				Preload("TrackRating", "user_id=?", user.ID).
				Find(&track)
			sub.PlayQueue.List[i] = spec.NewTCTrackByFolder(&track, track.Album)
			sub.PlayQueue.List[i].TranscodedContentType = transcodeMIME
			sub.PlayQueue.List[i].TranscodedSuffix = transcodeSuffix
		case specid.PodcastEpisode:
			pe := db.PodcastEpisode{}
			c.DB.
				Where("id=?", id.Value).
				Find(&pe)
			p := db.Podcast{}
			c.DB.
				Where("id=?", pe.PodcastID).
				Find(&p)
			sub.PlayQueue.List[i] = spec.NewTCPodcastEpisode(&pe, &p)
			sub.PlayQueue.List[i].TranscodedContentType = transcodeMIME
			sub.PlayQueue.List[i].TranscodedSuffix = transcodeSuffix
		}
	}
	return sub
}

func (c *Controller) ServeSavePlayQueue(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	tracks, err := params.GetIDList("id")
	if err != nil {
		return spec.NewError(10, "please provide some `id` parameters")
	}
	trackIDs := make([]specid.ID, 0, len(tracks))
	for _, id := range tracks {
		if (id.Type == specid.Track) || (id.Type == specid.PodcastEpisode) {
			trackIDs = append(trackIDs, id)
		}
	}
	if len(trackIDs) == 0 {
		return spec.NewError(10, "no track ids provided")
	}
	user := r.Context().Value(CtxUser).(*db.User)
	var queue db.PlayQueue
	c.DB.Where("user_id=?", user.ID).First(&queue)
	queue.UserID = user.ID
	queue.Current = params.GetOrID("current", specid.ID{}).String()
	queue.Position = params.GetOrInt("position", 0)
	queue.ChangedBy = params.GetOr("c", "") // must exist, middleware checks
	queue.SetItems(trackIDs)
	c.DB.Save(&queue)
	return spec.NewResponse()
}

func (c *Controller) ServeGetSong(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "provide an `id` parameter")
	}
	var track db.Track
	err = c.DB.
		Where("id=?", id.Value).
		Preload("Album").
		Preload("Album.TagArtist").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		First(&track).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(10, "couldn't find a track with that id")
	}
	sub := spec.NewResponse()
	sub.Track = spec.NewTrackByTags(&track, track.Album)
	return sub
}

func (c *Controller) ServeGetRandomSongs(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var tracks []*db.Track
	q := c.DB.DB.
		Limit(params.GetOrInt("size", 10)).
		Preload("Album").
		Preload("Album.TagArtist").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
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
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(10, "get random songs: %v", err)
	}
	sub := spec.NewResponse()
	sub.RandomTracks = &spec.RandomTracks{}
	sub.RandomTracks.List = make([]*spec.TrackChild, len(tracks))

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, track := range tracks {
		sub.RandomTracks.List[i] = spec.NewTrackByTags(track, track.Album)
		sub.RandomTracks.List[i].TranscodedContentType = transcodeMIME
		sub.RandomTracks.List[i].TranscodedSuffix = transcodeSuffix
	}
	return sub
}

func (c *Controller) ServeJukebox(r *http.Request) *spec.Response { // nolint:gocyclo
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	trackPaths := func(ids []specid.ID) ([]string, error) {
		var paths []string
		for _, id := range ids {
			var track db.Track
			if err := c.DB.Preload("Album").Preload("TrackStar", "user_id=?", user.ID).Preload("TrackRating", "user_id=?", user.ID).First(&track, id.Value).Error; err != nil {
				return nil, fmt.Errorf("find track by id: %w", err)
			}
			paths = append(paths, track.AbsPath())
		}
		return paths, nil
	}
	getSpecStatus := func() (*spec.JukeboxStatus, error) {
		status, err := c.Jukebox.GetStatus()
		if err != nil {
			return nil, fmt.Errorf("get status: %w", err)
		}
		return &spec.JukeboxStatus{
			CurrentIndex: status.CurrentIndex,
			Playing:      status.Playing,
			Gain:         float64(status.GainPct) / 100.0,
			Position:     status.Position,
		}, nil
	}
	getSpecPlaylist := func() ([]*spec.TrackChild, error) {
		var ret []*spec.TrackChild
		playlist, err := c.Jukebox.GetPlaylist()
		if err != nil {
			return nil, fmt.Errorf("get playlist: %w", err)
		}
		for _, path := range playlist {
			cwd, _ := os.Getwd()
			path, _ = filepath.Rel(cwd, path)
			var track db.Track
			err := c.DB.
				Preload("Album").
				Where(`(albums.root_dir || ? || albums.left_path || albums.right_path || ? || tracks.filename)=?`,
					string(filepath.Separator), string(filepath.Separator), path).
				Joins(`JOIN albums ON tracks.album_id=albums.id`).
				First(&track).
				Error
			if err != nil {
				return nil, fmt.Errorf("fetch track: %w", err)
			}
			ret = append(ret, spec.NewTrackByTags(&track, track.Album))
		}
		return ret, nil
	}

	switch act, _ := params.Get("action"); act {
	case "set":
		ids := params.GetOrIDList("id", nil)
		paths, err := trackPaths(ids)
		if err != nil {
			return spec.NewError(0, "error creating playlist items: %v", err)
		}
		if err := c.Jukebox.SetPlaylist(paths); err != nil {
			return spec.NewError(0, "error setting playlist: %v", err)
		}
	case "add":
		ids := params.GetOrIDList("id", nil)
		paths, err := trackPaths(ids)
		if err != nil {
			return spec.NewError(10, "error creating playlist items: %v", err)
		}
		if err := c.Jukebox.AppendToPlaylist(paths); err != nil {
			return spec.NewError(0, "error appending to playlist: %v", err)
		}
	case "clear":
		if err := c.Jukebox.ClearPlaylist(); err != nil {
			return spec.NewError(0, "error clearing playlist: %v", err)
		}
	case "remove":
		index, err := params.GetInt("index")
		if err != nil {
			return spec.NewError(10, "please provide an id for remove actions")
		}
		if err := c.Jukebox.RemovePlaylistIndex(index); err != nil {
			return spec.NewError(0, "error removing: %v", err)
		}
	case "stop":
		if err := c.Jukebox.Pause(); err != nil {
			return spec.NewError(0, "error stopping: %v", err)
		}
	case "start":
		if err := c.Jukebox.Play(); err != nil {
			return spec.NewError(0, "error starting: %v", err)
		}
	case "skip":
		index, err := params.GetInt("index")
		if err != nil {
			return spec.NewError(10, "please provide an index for skip actions")
		}
		offset, _ := params.GetInt("offset")
		if err := c.Jukebox.SkipToPlaylistIndex(index, offset); err != nil {
			return spec.NewError(0, "error skipping: %v", err)
		}
	case "get":
		specPlaylist, err := getSpecPlaylist()
		if err != nil {
			return spec.NewError(10, "error getting status tracks: %v", err)
		}
		status, err := getSpecStatus()
		if err != nil {
			return spec.NewError(10, "error getting status: %v", err)
		}
		sub := spec.NewResponse()
		sub.JukeboxPlaylist = &spec.JukeboxPlaylist{
			JukeboxStatus: status,
			List:          specPlaylist,
		}
		return sub
	case "setGain":
		gain, err := params.GetFloat("gain")
		if err != nil {
			return spec.NewError(10, "please provide a valid gain param")
		}
		if err := c.Jukebox.SetVolumePct(int(math.Min(gain, 1) * 100)); err != nil {
			return spec.NewError(0, "error setting gain: %v", err)
		}
	}
	// all actions except get are expected to return a status
	status, err := getSpecStatus()
	if err != nil {
		return spec.NewError(10, "error getting status: %v", err)
	}
	sub := spec.NewResponse()
	sub.JukeboxStatus = status
	return sub
}

func (c *Controller) ServeGetLyrics(r *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Lyrics = &spec.Lyrics{}
	return sub
}
