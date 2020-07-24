package ctrlsubsonic

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/encode"
)

// "raw" handlers are ones that don't always return a spec response.
// it could be a file, stream, etc. so you must either
//   a) write to response writer
//   b) return a non-nil spec.Response
//  _but not both_

func streamGetTransPref(dbc *db.DB, userID int, client string) db.TranscodePreference {
	pref := db.TranscodePreference{}
	dbc.
		Where("user_id=?", userID).
		Where("client COLLATE NOCASE IN (?)", []string{"*", client}).
		Order("client DESC"). // ensure "*" is last if it's there
		First(&pref)
	return pref
}

func streamGetTrack(dbc *db.DB, trackID int) (*db.Track, error) {
	track := db.Track{}
	err := dbc.
		Preload("Album").
		First(&track, trackID).
		Error
	return &track, err
}

func streamUpdateStats(dbc *db.DB, userID, albumID int) {
	play := db.Play{
		AlbumID: albumID,
		UserID:  userID,
	}
	dbc.
		Where(play).
		First(&play)
	play.Time = time.Now() // for getAlbumList?type=recent
	play.Count++           // for getAlbumList?type=frequent
	dbc.Save(&play)
}

const (
	coverDefaultSize = 600
	coverCacheFormat = "png"
)

var (
	errCoverNotFound = errors.New("could not find a cover with that id")
	errCoverEmpty    = errors.New("no cover found for that folder")
)

func coverGetPath(dbc *db.DB, musicPath string, id int) (string, error) {
	folder := &db.Album{}
	err := dbc.DB.
		Select("id, left_path, right_path, cover").
		First(folder, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return "", errCoverNotFound
	}
	if folder.Cover == "" {
		return "", errCoverEmpty
	}
	return path.Join(
		musicPath,
		folder.LeftPath,
		folder.RightPath,
		folder.Cover,
	), nil
}

func coverScaleAndSave(absPath, cachePath string, size int) error {
	src, err := imaging.Open(absPath)
	if err != nil {
		return fmt.Errorf("resizing `%s`: %w", absPath, err)
	}
	width := size
	if width > src.Bounds().Dx() {
		// don't upscale images
		width = src.Bounds().Dx()
	}
	err = imaging.Save(imaging.Resize(src, width, 0, imaging.Lanczos), cachePath)
	if err != nil {
		return fmt.Errorf("caching `%s`: %w", cachePath, err)
	}
	return nil
}

func (c *Controller) ServeGetCoverArt(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	size := params.GetOrInt("size", coverDefaultSize)
	cachePath := path.Join(
		c.CoverCachePath,
		fmt.Sprintf("%s-%d.%s", id.String(), size, coverCacheFormat),
	)
	_, err = os.Stat(cachePath)
	switch {
	case os.IsNotExist(err):
		coverPath, err := coverGetPath(c.DB, c.MusicPath, id.Value)
		if err != nil {
			return spec.NewError(10, "couldn't find cover %q: %v", id, err)
		}
		if err := coverScaleAndSave(coverPath, cachePath, size); err != nil {
			log.Printf("error scaling cover: %v", err)
			return nil
		}
	case err != nil:
		log.Printf("error stating `%s`: %v", cachePath, err)
		return nil
	}
	http.ServeFile(w, r, cachePath)
	return nil
}

func (c *Controller) ServeStream(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	track, err := streamGetTrack(c.DB, id.Value)
	if err != nil {
		return spec.NewError(70, "media with id `%d` was not found", id.Value)
	}
	user := r.Context().Value(CtxUser).(*db.User)
	defer streamUpdateStats(c.DB, user.ID, track.Album.ID)
	pref := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	trackPath := path.Join(c.MusicPath, track.RelPath())
	//
	onInvalidProfile := func() error {
		log.Printf("serving raw %q\n", track.Filename)
		w.Header().Set("Content-Type", track.MIME())
		http.ServeFile(w, r, trackPath)
		return nil
	}
	onCacheHit := func(profile encode.Profile, path string) error {
		log.Printf("serving transcode `%s`: cache [%s/%dk] hit!\n",
			track.Filename, profile.Format, profile.Bitrate)
		http.ServeFile(w, r, path)
		return nil
	}
	onCacheMiss := func(profile encode.Profile) (io.Writer, error) {
		log.Printf("serving transcode `%s`: cache [%s/%dk] miss!\n",
			track.Filename, profile.Format, profile.Bitrate)
		return w, nil
	}
	encodeOptions := encode.Options{
		TrackPath:        trackPath,
		CachePath:        c.CachePath,
		ProfileName:      pref.Profile,
		PreferredBitrate: params.GetOrInt("maxBitRate", 0),
		OnInvalidProfile: onInvalidProfile,
		OnCacheHit:       onCacheHit,
		OnCacheMiss:      onCacheMiss,
	}
	if err := encode.Encode(encodeOptions); err != nil {
		log.Printf("serving transcode `%s`: error: %v\n", track.Filename, err)
	}
	return nil
}

func (c *Controller) ServeDownload(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	track, err := streamGetTrack(c.DB, id.Value)
	if err != nil {
		return spec.NewError(70, "media with id `%s` was not found", id)
	}
	log.Printf("serving raw %q\n", track.Filename)
	w.Header().Set("Content-Type", track.MIME())
	trackPath := path.Join(c.MusicPath, track.RelPath())
	http.ServeFile(w, r, trackPath)
	return nil
}
