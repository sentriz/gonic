package ctrlsubsonic

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/scrobble"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
)

func (c *Controller) ServeGetLicence(_ *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Licence = &spec.Licence{
		Valid: true,
	}
	return sub
}

func (c *Controller) ServePing(_ *http.Request) *spec.Response {
	return spec.NewResponse()
}

func (c *Controller) ServeGetOpenSubsonicExtensions(_ *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.OpenSubsonicExtensions = &spec.OpenSubsonicExtensions{
		{Name: "transcodeOffset", Versions: []int{1}},
		{Name: "formPost", Versions: []int{1}},
	}
	return sub
}

func (c *Controller) ServeScrobble(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide a `id` parameter")
	}

	optStamp := params.GetOrTime("time", time.Now())
	optSubmission := params.GetOrBool("submission", true)

	var scrobbleTrack scrobble.Track

	switch id.Type {
	case specid.Track:
		var track db.Track
		if err := c.dbc.Preload("Album").Preload("Album.Artists").First(&track, id.Value).Error; err != nil {
			return spec.NewError(0, "error finding track: %v", err)
		}
		if track.Album == nil {
			return spec.NewError(0, "track has no album %d", track.ID)
		}

		scrobbleTrack.Track = track.TagTitle
		scrobbleTrack.Artist = track.TagTrackArtist
		scrobbleTrack.Album = track.Album.TagTitle
		scrobbleTrack.AlbumArtist = track.Album.TagAlbumArtist
		scrobbleTrack.TrackNumber = uint(track.TagTrackNumber)
		scrobbleTrack.Duration = time.Second * time.Duration(track.Length)
		if _, err := uuid.Parse(track.TagBrainzID); err == nil {
			scrobbleTrack.MusicBrainzID = track.TagBrainzID
		}
		if _, err := uuid.Parse(track.Album.TagBrainzID); err == nil {
			scrobbleTrack.MusicBrainzReleaseID = track.Album.TagBrainzID
		}

		if err := scrobbleStatsUpdateTrack(c.dbc, &track, user.ID, optStamp); err != nil {
			return spec.NewError(0, "error updating stats: %v", err)
		}

	case specid.PodcastEpisode:
		var podcastEpisode db.PodcastEpisode
		if err := c.dbc.Preload("Podcast").First(&podcastEpisode, id.Value).Error; err != nil {
			return spec.NewError(0, "error finding podcast episode: %v", err)
		}

		if err := scrobbleStatsUpdatePodcastEpisode(c.dbc, id.Value); err != nil {
			return spec.NewError(0, "error updating stats: %v", err)
		}
	}

	if scrobbleTrack.Track == "" {
		return spec.NewResponse()
	}

	var wg sync.WaitGroup

	scrobbleErrs := make([]error, len(c.scrobblers))
	for i := range c.scrobblers {
		if !c.scrobblers[i].IsUserAuthenticated(*user) {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := c.scrobblers[i].Scrobble(*user, scrobbleTrack, optStamp, optSubmission); err != nil {
				scrobbleErrs[i] = err
			}
		}(i)
	}

	wg.Wait()

	if err := errors.Join(scrobbleErrs...); err != nil {
		return spec.NewError(0, "error when submitting: %v", err)
	}

	return spec.NewResponse()
}

func (c *Controller) ServeGetMusicFolders(_ *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.MusicFolders = &spec.MusicFolders{}
	for i, mp := range c.musicPaths {
		alias := mp.Alias
		if alias == "" {
			alias = filepath.Base(mp.Path)
		}
		sub.MusicFolders.List = append(sub.MusicFolders.List, &spec.MusicFolder{ID: i, Name: alias})
	}
	return sub
}

func (c *Controller) ServeStartScan(r *http.Request) *spec.Response {
	go func() {
		if _, err := c.scanner.ScanAndClean(scanner.ScanOptions{}); err != nil {
			log.Printf("error while scanning: %v\n", err)
		}
	}()
	return c.ServeGetScanStatus(r)
}

func (c *Controller) ServeGetScanStatus(_ *http.Request) *spec.Response {
	var trackCount int
	if err := c.dbc.Model(db.Track{}).Count(&trackCount).Error; err != nil {
		return spec.NewError(0, "error finding track count: %v", err)
	}

	sub := spec.NewResponse()
	sub.ScanStatus = &spec.ScanStatus{
		Scanning: c.scanner.IsScanning(),
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
		JukeboxRole:       c.jukebox != nil,
		PodcastRole:       c.podcasts != nil,
		DownloadRole:      true,
		ScrobblingEnabled: hasLastFM || hasListenBrainz,
		Folder:            []int{1},
	}
	return sub
}

func (c *Controller) ServeNotFound(_ *http.Request) *spec.Response {
	return spec.NewError(70, "view not found")
}

func (c *Controller) ServeGetPlayQueue(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var queue db.PlayQueue
	err := c.dbc.
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

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for i, id := range trackIDs {
		switch id.Type {
		case specid.Track:
			track := db.Track{}
			c.dbc.
				Where("id=?", id.Value).
				Preload("Album").
				Preload("Artists").
				Preload("TrackStar", "user_id=?", user.ID).
				Preload("TrackRating", "user_id=?", user.ID).
				Find(&track)
			sub.PlayQueue.List[i] = spec.NewTCTrackByFolder(&track, track.Album)
			sub.PlayQueue.List[i].TranscodeMeta = transcodeMeta
		case specid.PodcastEpisode:
			pe := db.PodcastEpisode{}
			c.dbc.
				Where("id=?", id.Value).
				Find(&pe)
			sub.PlayQueue.List[i] = spec.NewTCPodcastEpisode(&pe)
			sub.PlayQueue.List[i].TranscodeMeta = transcodeMeta
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
	c.dbc.Where("user_id=?", user.ID).First(&queue)
	queue.UserID = user.ID
	queue.Current = params.GetOrID("current", specid.ID{}).String()
	queue.Position = params.GetOrInt("position", 0)
	queue.ChangedBy = params.GetOr("c", "") // must exist, middleware checks
	queue.SetItems(trackIDs)
	c.dbc.Save(&queue)
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
	err = c.dbc.
		Where("id=?", id.Value).
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		First(&track).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "couldn't find a track with that id")
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	sub := spec.NewResponse()
	sub.Track = spec.NewTrackByTags(&track, track.Album)

	sub.Track.TranscodeMeta = transcodeMeta

	return sub
}

func (c *Controller) ServeGetRandomSongs(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var tracks []*db.Track
	q := c.dbc.DB.
		Limit(params.GetOrInt("size", 10)).
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
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
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(10, "get random songs: %v", err)
	}
	sub := spec.NewResponse()
	sub.RandomTracks = &spec.RandomTracks{}
	sub.RandomTracks.List = make([]*spec.TrackChild, len(tracks))

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for i, track := range tracks {
		sub.RandomTracks.List[i] = spec.NewTrackByTags(track, track.Album)
		sub.RandomTracks.List[i].TranscodeMeta = transcodeMeta
	}
	return sub
}

var errNotATrack = errors.New("not a track")

func (c *Controller) ServeJukebox(r *http.Request) *spec.Response { // nolint:gocyclo
	if c.jukebox == nil {
		return spec.NewError(0, "jukebox not enabled")
	}

	params := r.Context().Value(CtxParams).(params.Params)
	trackPaths := func(ids []specid.ID) ([]string, error) {
		var paths []string
		for _, id := range ids {
			r, err := specidpaths.Locate(c.dbc, id)
			if err != nil {
				return nil, fmt.Errorf("find track by id: %w", err)
			}
			paths = append(paths, r.AbsPath())
		}
		return paths, nil
	}
	getSpecStatus := func() (*spec.JukeboxStatus, error) {
		status, err := c.jukebox.GetStatus()
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
		playlist, err := c.jukebox.GetPlaylist()
		if err != nil {
			return nil, fmt.Errorf("get playlist: %w", err)
		}
		for _, path := range playlist {
			file, err := specidpaths.Lookup(c.dbc, MusicPaths(c.musicPaths), c.podcastsPath, path)
			if err != nil {
				return nil, fmt.Errorf("fetch track: %w", err)
			}
			track, ok := file.(*db.Track)
			if !ok {
				return nil, fmt.Errorf("%q: %w", path, errNotATrack)
			}
			ret = append(ret, spec.NewTrackByTags(track, track.Album))
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
		if err := c.jukebox.SetPlaylist(paths); err != nil {
			return spec.NewError(0, "error setting playlist: %v", err)
		}
	case "add":
		ids := params.GetOrIDList("id", nil)
		paths, err := trackPaths(ids)
		if err != nil {
			return spec.NewError(10, "error creating playlist items: %v", err)
		}
		if err := c.jukebox.AppendToPlaylist(paths); err != nil {
			return spec.NewError(0, "error appending to playlist: %v", err)
		}
	case "clear":
		if err := c.jukebox.ClearPlaylist(); err != nil {
			return spec.NewError(0, "error clearing playlist: %v", err)
		}
	case "remove":
		index, err := params.GetInt("index")
		if err != nil {
			return spec.NewError(10, "please provide an id for remove actions")
		}
		if err := c.jukebox.RemovePlaylistIndex(index); err != nil {
			return spec.NewError(0, "error removing: %v", err)
		}
	case "stop":
		if err := c.jukebox.Pause(); err != nil {
			return spec.NewError(0, "error stopping: %v", err)
		}
	case "start":
		if err := c.jukebox.Play(); err != nil {
			return spec.NewError(0, "error starting: %v", err)
		}
	case "skip":
		index, err := params.GetInt("index")
		if err != nil {
			return spec.NewError(10, "please provide an index for skip actions")
		}
		offset, _ := params.GetInt("offset")
		if err := c.jukebox.SkipToPlaylistIndex(index, offset); err != nil {
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
		if err := c.jukebox.SetVolumePct(int(math.Min(gain, 1) * 100)); err != nil {
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

func (c *Controller) ServeGetLyrics(_ *http.Request) *spec.Response {
	sub := spec.NewResponse()
	sub.Lyrics = &spec.Lyrics{}
	return sub
}

func scrobbleStatsUpdateTrack(dbc *db.DB, track *db.Track, userID int, playTime time.Time) error {
	var play db.Play
	if err := dbc.Where("album_id=? AND user_id=?", track.AlbumID, userID).First(&play).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find stat: %w", err)
	}

	play.AlbumID = track.AlbumID
	play.UserID = userID
	play.Count++ // for getAlbumList?type=frequent
	play.Length += track.Length
	if playTime.After(play.Time) {
		play.Time = playTime // for getAlbumList?type=recent
	}

	if err := dbc.Save(&play).Error; err != nil {
		return fmt.Errorf("save stat: %w", err)
	}
	return nil
}

func scrobbleStatsUpdatePodcastEpisode(dbc *db.DB, peID int) error {
	var pe db.PodcastEpisode
	if err := dbc.Where("id=?", peID).First(&pe).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find podcast episode: %w", err)
	}

	pe.ModifiedAt = time.Now()

	if err := dbc.Save(&pe).Error; err != nil {
		return fmt.Errorf("save podcast episode: %w", err)
	}
	return nil
}

func getMusicFolder(musicPaths []MusicPath, p params.Params) string {
	idx, err := p.GetInt("musicFolderId")
	if err != nil {
		return ""
	}
	if idx < 0 || idx >= len(musicPaths) {
		return os.DevNull
	}
	return musicPaths[idx].Path
}

func lowerUDecOrHash(in string) string {
	inRunes := []rune(in)
	if len(inRunes) == 0 {
		return ""
	}
	lower := unicode.ToLower(inRunes[0])
	if !unicode.IsLetter(lower) {
		return "#"
	}
	return string(lower)
}
