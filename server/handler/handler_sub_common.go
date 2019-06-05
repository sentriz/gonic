package handler

import (
	"net/http"
	"os"
	"path"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"
	"github.com/rainycape/unidecode"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/scanner"
	"github.com/sentriz/gonic/server/lastfm"
	"github.com/sentriz/gonic/server/subsonic"
)

func indexOf(s string) rune {
	first := string(s[0])
	c := rune(unidecode.Unidecode(first)[0])
	if !unicode.IsLetter(c) {
		return '#'
	}
	return c
}

func (c *Controller) Stream(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var track model.Track
	err = c.DB.
		Preload("Album").
		First(&track, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		respondError(w, r, 70, "media with id `%d` was not found", id)
		return
	}
	absPath := path.Join(
		c.MusicPath,
		track.Album.LeftPath,
		track.Album.RightPath,
		track.Filename,
	)
	file, err := os.Open(absPath)
	if err != nil {
		respondError(w, r, 0, "error while streaming media: %v", err)
		return
	}
	stat, _ := file.Stat()
	http.ServeContent(w, r, absPath, stat.ModTime(), file)
	//
	// after we've served the file, mark the album as played
	user := r.Context().Value(contextUserKey).(*model.User)
	play := model.Play{
		AlbumID: track.Album.ID,
		UserID:  user.ID,
	}
	c.DB.
		Where(play).
		First(&play)
	play.Time = time.Now() // for getAlbumList?type=recent
	play.Count++           // for getAlbumList?type=frequent
	c.DB.Save(&play)
}

func (c *Controller) GetCoverArt(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	var folder model.Album
	err = c.DB.
		Select("id, path, cover").
		First(&folder, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		respondError(w, r, 10, "could not find a cover with that id")
		return
	}
	if folder.Cover == "" {
		respondError(w, r, 10, "no cover found for that folder")
		return
	}
	absPath := path.Join(
		c.MusicPath,
		folder.RightPath,
		folder.LeftPath,
		folder.Cover,
	)
	http.ServeFile(w, r, absPath)
}

func (c *Controller) GetLicence(w http.ResponseWriter, r *http.Request) {
	sub := subsonic.NewResponse()
	sub.Licence = &subsonic.Licence{
		Valid: true,
	}
	respond(w, r, sub)
}

func (c *Controller) Ping(w http.ResponseWriter, r *http.Request) {
	sub := subsonic.NewResponse()
	respond(w, r, sub)
}

func (c *Controller) Scrobble(w http.ResponseWriter, r *http.Request) {
	id, err := getIntParam(r, "id")
	if err != nil {
		respondError(w, r, 10, "please provide an `id` parameter")
		return
	}
	// fetch user to get lastfm session
	user := r.Context().Value(contextUserKey).(*model.User)
	if user.LastFMSession == "" {
		respondError(w, r, 0, "you don't have a last.fm session")
		return
	}
	// fetch track for getting info to send to last.fm function
	var track model.Track
	c.DB.
		Preload("Album").
		Preload("Artist").
		First(&track, id)
	// scrobble with above info
	err = lastfm.Scrobble(
		c.GetSetting("lastfm_api_key"),
		c.GetSetting("lastfm_secret"),
		user.LastFMSession,
		&track,
		// clients will provide time in miliseconds, so use that or
		// instead convert UnixNano to miliseconds
		getIntParamOr(r, "time", int(time.Now().UnixNano()/1e6)),
		getStrParamOr(r, "submission", "true") != "false",
	)
	if err != nil {
		respondError(w, r, 0, "error when submitting: %v", err)
		return
	}
	sub := subsonic.NewResponse()
	respond(w, r, sub)
}

func (c *Controller) GetMusicFolders(w http.ResponseWriter, r *http.Request) {
	folders := &subsonic.MusicFolders{}
	folders.List = []*subsonic.MusicFolder{
		{ID: 1, Name: "music"},
	}
	sub := subsonic.NewResponse()
	sub.MusicFolders = folders
	respond(w, r, sub)
}

func (c *Controller) StartScan(w http.ResponseWriter, r *http.Request) {
	scanC := scanner.New(c.DB, c.MusicPath)
	go scanC.Start()
	c.GetScanStatus(w, r)
}

func (c *Controller) GetScanStatus(w http.ResponseWriter, r *http.Request) {
	var trackCount int
	c.DB.Model(&model.Track{}).Count(&trackCount)
	sub := subsonic.NewResponse()
	sub.ScanStatus = &subsonic.ScanStatus{
		Scanning: atomic.LoadInt32(&scanner.IsScanning) == 1,
		Count:    trackCount,
	}
	respond(w, r, sub)
}

func (c *Controller) NotFound(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, 0, "unknown route")
}
