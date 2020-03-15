package ctrlsubsonic

import (
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/server/ctrlsubsonic/params"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
	"senan.xyz/g/gonic/server/encode"
)

// "raw" handlers are ones that don't always return a spec response.
// it could be a file, stream, etc. so you must either
//   a) write to response writer
//   b) return a non-nil spec.Response
//  _but not both_

func (c *Controller) ServeGetCoverArt(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	folder := &db.Album{}
	err = c.DB.
		Select("id, left_path, right_path, cover").
		First(folder, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(10, "could not find a cover with that id")
	}
	if folder.Cover == "" {
		return spec.NewError(10, "no cover found for that folder")
	}
	absPath := path.Join(
		c.MusicPath,
		folder.LeftPath,
		folder.RightPath,
		folder.Cover,
	)
	http.ServeFile(w, r, absPath)
	return nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

type serveTrackOptions struct {
	track      *db.Track
	pref       *db.TranscodePreference
	maxBitrate int
	cachePath  string
	musicPath  string
}

func serveTrackRaw(w http.ResponseWriter, r *http.Request, opts serveTrackOptions) {
	log.Printf("serving raw %q\n", opts.track.Filename)
	w.Header().Set("Content-Type", opts.track.MIME())
	trackPath := path.Join(opts.musicPath, opts.track.RelPath())
	http.ServeFile(w, r, trackPath)
}

func serveTrackEncode(w http.ResponseWriter, r *http.Request, opts serveTrackOptions) {
	profile := encode.Profiles[opts.pref.Profile]
	bitrate := encode.GetBitrate(opts.maxBitrate, profile)
	trackPath := path.Join(opts.musicPath, opts.track.RelPath())
	cacheKey := encode.CacheKey(trackPath, opts.pref.Profile, bitrate)
	cacheFile := path.Join(opts.cachePath, cacheKey)
	if fileExists(cacheFile) {
		log.Printf("serving transcode `%s`: cache [%s/%s] hit!\n", opts.track.Filename, profile.Format, bitrate)
		http.ServeFile(w, r, cacheFile)
		return
	}
	log.Printf("serving transcode `%s`: cache [%s/%s] miss!\n", opts.track.Filename, profile.Format, bitrate)
	if err := encode.Encode(w, trackPath, cacheFile, profile, bitrate); err != nil {
		log.Printf("error encoding %q: %v\n", trackPath, err)
		return
	}
	log.Printf("serving transcode `%s`: encoded to [%s/%s] successfully\n",
		opts.track.Filename, profile.Format, bitrate)
}

func (c *Controller) ServeStream(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	track := &db.Track{}
	err = c.DB.
		Preload("Album").
		First(track, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(70, "media with id `%d` was not found", id)
	}
	user := r.Context().Value(CtxUser).(*db.User)
	defer func() {
		play := db.Play{
			AlbumID: track.Album.ID,
			UserID:  user.ID,
		}
		c.DB.
			Where(play).
			First(&play)
		play.Time = time.Now() // for getAlbumList?type=recent
		play.Count++           // for getAlbumList?type=frequent
		c.DB.Save(&play)
	}()
	client := params.Get("c")
	servOpts := serveTrackOptions{
		track:     track,
		musicPath: c.MusicPath,
	}
	pref := &db.TranscodePreference{}
	err = c.DB.
		Where("user_id=?", user.ID).
		Where("client COLLATE NOCASE IN (?)", []string{"*", client}).
		Order("client DESC"). // ensure "*" is last if it's there
		First(pref).
		Error
	if gorm.IsRecordNotFoundError(err) {
		serveTrackRaw(w, r, servOpts)
		return nil
	}
	servOpts.pref = pref
	servOpts.maxBitrate = params.GetIntOr("maxBitRate", 0)
	servOpts.cachePath = c.CachePath
	serveTrackEncode(w, r, servOpts)
	return nil
}

func (c *Controller) ServeDownload(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	track := &db.Track{}
	err = c.DB.
		Preload("Album").
		First(track, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(70, "media with id `%d` was not found", id)
	}
	serveTrackRaw(w, r, serveTrackOptions{
		track:     track,
		musicPath: c.MusicPath,
	})
	return nil
}
