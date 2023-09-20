package ctrlsubsonic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/artistinfocache"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specidpaths"
	"go.senan.xyz/gonic/transcode"
)

// "raw" handlers are ones that don't always return a spec response.
// it could be a file, stream, etc. so you must either
//   a) write to response writer
//   b) return a non-nil spec.Response
//  _but not both_

func streamGetTransodePreference(dbc *db.DB, userID int, client string) (*db.TranscodePreference, error) {
	var pref db.TranscodePreference
	err := dbc.
		Where("user_id=?", userID).
		Where("client COLLATE NOCASE IN (?)", []string{"*", client}).
		Order("client DESC"). // ensure "*" is last if it's there
		First(&pref).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find transcode preference: %w", err)
	}
	return &pref, nil
}

func streamGetTranscodeMeta(dbc *db.DB, userID int, client string) spec.TranscodeMeta {
	pref, _ := streamGetTransodePreference(dbc, userID, client)
	if pref == nil {
		return spec.TranscodeMeta{}
	}
	profile, ok := transcode.UserProfiles[pref.Profile]
	if !ok {
		return spec.TranscodeMeta{}
	}
	return spec.TranscodeMeta{
		TranscodedContentType: profile.MIME(),
		TranscodedSuffix:      profile.Suffix(),
	}
}

var errUnknownMediaType = fmt.Errorf("media type is unknown")

func streamUpdateStats(dbc *db.DB, userID int, track *db.Track, playTime time.Time) error {
	var play db.Play
	err := dbc.
		Where("album_id=? AND user_id=?", track.AlbumID, userID).
		First(&play).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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

func streamUpdatePodcastEpisodeStats(dbc *db.DB, peID int) error {
	var pe db.PodcastEpisode
	err := dbc.
		Where("id=?", peID).
		First(&pe).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find podcast episode: %w", err)
	}

	pe.ModifiedAt = time.Now()

	if err := dbc.Save(&pe).Error; err != nil {
		return fmt.Errorf("save podcast episode: %w", err)
	}
	return nil
}

const (
	coverDefaultSize = 600
	coverCacheFormat = "png"
)

var (
	errCoverNotFound = errors.New("could not find a cover with that id")
	errCoverEmpty    = errors.New("no cover found")
)

// TODO: can we use specidpaths.Locate here?
func coverFor(dbc *db.DB, artistInfoCache *artistinfocache.ArtistInfoCache, id specid.ID) (io.ReadCloser, error) {
	switch id.Type {
	case specid.Album:
		return coverForAlbum(dbc, id.Value)
	case specid.Artist:
		return coverForArtist(artistInfoCache, id.Value)
	case specid.Podcast:
		return coverForPodcast(dbc, id.Value)
	case specid.PodcastEpisode:
		return coverGetPathPodcastEpisode(dbc, id.Value)
	default:
		return nil, errCoverNotFound
	}
}

func coverForAlbum(dbc *db.DB, id int) (*os.File, error) {
	var folder db.Album
	err := dbc.DB.
		Select("id, root_dir, left_path, right_path, cover").
		First(&folder, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select album: %w", err)
	}
	if folder.Cover == "" {
		return nil, errCoverEmpty
	}
	return os.Open(filepath.Join(folder.RootDir, folder.LeftPath, folder.RightPath, folder.Cover))
}

func coverForArtist(artistInfoCache *artistinfocache.ArtistInfoCache, id int) (io.ReadCloser, error) {
	info, err := artistInfoCache.Get(context.Background(), id)
	if err != nil {
		return nil, fmt.Errorf("get artist info from cache: %w", err)
	}
	if info.ImageURL == "" {
		return nil, fmt.Errorf("%w: cache miss", errCoverEmpty)
	}
	resp, err := http.Get(info.ImageURL)
	if err != nil {
		return nil, fmt.Errorf("req image from lastfm: %w", err)
	}
	return resp.Body, nil
}

func coverForPodcast(dbc *db.DB, id int) (*os.File, error) {
	var podcast db.Podcast
	err := dbc.
		First(&podcast, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select podcast: %w", err)
	}
	if podcast.Image == "" {
		return nil, errCoverEmpty
	}
	return os.Open(filepath.Join(podcast.RootDir, podcast.Image))
}

func coverGetPathPodcastEpisode(dbc *db.DB, id int) (*os.File, error) {
	var pe db.PodcastEpisode
	err := dbc.
		Preload("Podcast").
		First(&pe, id).
		Error
	if err != nil {
		return nil, fmt.Errorf("select episode: %w", err)
	}
	if pe.Podcast == nil || pe.Podcast.Image == "" {
		return nil, errCoverEmpty
	}
	return os.Open(filepath.Join(pe.Podcast.RootDir, pe.Podcast.Image))
}

func coverScaleAndSave(reader io.Reader, cachePath string, size int) error {
	src, err := imaging.Decode(reader)
	if err != nil {
		return fmt.Errorf("resizing: %w", err)
	}
	width := size
	if width > src.Bounds().Dx() {
		// don't upscale images
		width = src.Bounds().Dx()
	}
	if err = imaging.Save(imaging.Resize(src, width, 0, imaging.Lanczos), cachePath); err != nil {
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
	cachePath := filepath.Join(
		c.CacheCoverPath,
		fmt.Sprintf("%s-%d.%s", id.String(), size, coverCacheFormat),
	)
	_, err = os.Stat(cachePath)
	switch {
	case os.IsNotExist(err):
		reader, err := coverFor(c.DB, c.ArtistInfoCache, id)
		if err != nil {
			return spec.NewError(10, "couldn't find cover `%s`: %v", id, err)
		}
		defer reader.Close()

		if err := coverScaleAndSave(reader, cachePath, size); err != nil {
			log.Printf("error scaling cover: %v", err)
			return nil
		}
	case err != nil:
		log.Printf("error stating `%s`: %v", cachePath, err)
		return nil
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, cachePath)

	return nil
}

func (c *Controller) ServeStream(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	file, err := specidpaths.Locate(c.DB, id)
	if err != nil {
		return spec.NewError(0, "error looking up id %s: %v", id, err)
	}

	audioFile, ok := file.(db.AudioFile)
	if !ok {
		return spec.NewError(0, "type of id does not contain audio")
	}

	if track, ok := audioFile.(*db.Track); ok && track.Album != nil {
		defer func() {
			if err := streamUpdateStats(c.DB, user.ID, track, time.Now()); err != nil {
				log.Printf("error updating track status: %v", err)
			}
		}()
	}

	if pe, ok := audioFile.(*db.PodcastEpisode); ok {
		defer func() {
			if err := streamUpdatePodcastEpisodeStats(c.DB, pe.ID); err != nil {
				log.Printf("error updating podcast episode status: %v", err)
			}
		}()
	}

	maxBitRate, _ := params.GetInt("maxBitRate")
	format, _ := params.Get("format")

	if format == "raw" || maxBitRate >= audioFile.AudioBitrate() {
		http.ServeFile(w, r, file.AbsPath())
		return nil
	}

	pref, err := streamGetTransodePreference(c.DB, user.ID, params.GetOr("c", ""))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(0, "couldn't find transcode preference: %v", err)
	}
	if pref == nil {
		http.ServeFile(w, r, file.AbsPath())
		return nil
	}

	profile, ok := transcode.UserProfiles[pref.Profile]
	if !ok {
		return spec.NewError(0, "unknown transcode user profile %q", pref.Profile)
	}
	if maxBitRate > 0 && int(profile.BitRate()) > maxBitRate {
		profile = transcode.WithBitrate(profile, transcode.BitRate(maxBitRate))
	}

	log.Printf("trancoding to %q with max bitrate %dk", profile.MIME(), profile.BitRate())

	w.Header().Set("Content-Type", profile.MIME())
	if err := c.Transcoder.Transcode(r.Context(), profile, file.AbsPath(), w); err != nil && !errors.Is(err, transcode.ErrFFmpegKilled) {
		return spec.NewError(0, "error transcoding: %v", err)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func (c *Controller) ServeGetAvatar(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	username, err := params.Get("username")
	if err != nil {
		return spec.NewError(10, "please provide an `username` parameter")
	}
	reqUser := c.DB.GetUserByName(username)
	if (user != reqUser) && !user.IsAdmin {
		return spec.NewError(50, "user not admin")
	}
	http.ServeContent(w, r, "", time.Now(), bytes.NewReader(reqUser.Avatar))
	return nil
}
