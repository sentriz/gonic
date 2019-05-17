package handler

import (
	"fmt"
	"net/http"
	"os"
	"time"
	"unicode"

	"github.com/rainycape/unidecode"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/lastfm"
	"github.com/sentriz/gonic/subsonic"
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
	var track db.Track
	c.DB.
		Preload("Album").
		Preload("Folder").
		First(&track, id)
	if track.Path == "" {
		respondError(w, r, 70, fmt.Sprintf("media with id `%d` was not found", id))
		return
	}
	file, err := os.Open(track.Path)
	if err != nil {
		respondError(w, r, 0, fmt.Sprintf("error while streaming media: %v", err))
		return
	}
	stat, _ := file.Stat()
	http.ServeContent(w, r, track.Path, stat.ModTime(), file)
	//
	// after we've served the file, mark the album as played
	user := r.Context().Value(contextUserKey).(*db.User)
	play := db.Play{
		AlbumID:  track.Album.ID,
		FolderID: track.Folder.ID,
		UserID:   user.ID,
	}
	c.DB.Where(play).First(&play)
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
	var cover db.Cover
	c.DB.First(&cover, id)
	w.Write(cover.Image)
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
	user := r.Context().Value(contextUserKey).(*db.User)
	if user.LastFMSession == "" {
		respondError(w, r, 0, fmt.Sprintf("no last.fm session for this user: %v", err))
		return
	}
	// fetch track for getting info to send to last.fm function
	var track db.Track
	c.DB.
		Preload("Album").
		Preload("AlbumArtist").
		First(&track, id)
	// get time from args or use now
	time := getIntParamOr(r, "time", int(time.Now().Unix()))
	// get submission, where the default is true. we will
	// check if it's false later
	submission := getStrParamOr(r, "submission", "true")
	// scrobble with above info
	err = lastfm.Scrobble(
		c.GetSetting("lastfm_api_key"),
		c.GetSetting("lastfm_secret"),
		user.LastFMSession,
		&track,
		time,
		submission != "false",
	)
	if err != nil {
		respondError(w, r, 0, fmt.Sprintf("error when submitting: %v", err))
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

func (c *Controller) NotFound(w http.ResponseWriter, r *http.Request) {
	respondError(w, r, 0, "unknown route")
}
