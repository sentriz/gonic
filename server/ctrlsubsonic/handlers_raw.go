package ctrlsubsonic

import (
	"io"
	"log"
	"net/http"
	"path"
	"time"

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

func (c *Controller) ServeGetCoverArt(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	folder := &db.Album{}
	err = c.DB.
		Select("id, left_path, right_path, cover").
		First(folder, id.Value).
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
