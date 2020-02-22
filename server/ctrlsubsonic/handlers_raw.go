package ctrlsubsonic

import (
	"net/http"
	"path"
	"time"

	"github.com/jinzhu/gorm"

	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/mime"
	"senan.xyz/g/gonic/server/ctrlsubsonic/params"
	"senan.xyz/g/gonic/server/ctrlsubsonic/spec"
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

	client := params.GetOr("c", "generic")
	bitrate, err := params.GetInt("maxBitRate")
	if err != nil {
		bitrate = 0
	}

	absPath := path.Join(
		c.MusicPath,
		track.Album.LeftPath,
		track.Album.RightPath,
		track.Filename,
	)
	streamTrack(w, r, absPath, client, bitrate, c.CachePath)

	//
	// after we've served the file, mark the album as played
	user := r.Context().Value(CtxUser).(*db.User)
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
	return nil
}

func (c *Controller) ServeDownload(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetInt("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	track := &model.Track{}
	err = c.DB.
		Preload("Album").
		First(track, id).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(70, "media with id `%d` was not found", id)
	}

	absPath := path.Join(
		c.MusicPath,
		track.Album.LeftPath,
		track.Album.RightPath,
		track.Filename,
	)
	if mime, ok := mime.Types[track.Ext()]; ok {
		w.Header().Set("Content-Type", mime)
	}
	http.ServeFile(w, r, absPath)

	//
	// We don't need to mark album/track as played
	// if user just downloads a track, so bail out here:
	return nil
}
