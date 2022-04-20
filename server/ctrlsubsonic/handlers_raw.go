package ctrlsubsonic

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/disintegration/imaging"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/transcode"
)

// "raw" handlers are ones that don't always return a spec response.
// it could be a file, stream, etc. so you must either
//   a) write to response writer
//   b) return a non-nil spec.Response
//  _but not both_

func streamGetTransPref(dbc *db.DB, userID int, client string) (*db.TranscodePreference, error) {
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

func streamGetTrack(dbc *db.DB, trackID int) (*db.Track, error) {
	var track db.Track
	err := dbc.
		Preload("Album").
		First(&track, trackID).
		Error
	if err != nil {
		return nil, fmt.Errorf("find track: %w", err)
	}
	return &track, nil
}

func streamGetPodcast(dbc *db.DB, podcastID int) (*db.PodcastEpisode, error) {
	var podcast db.PodcastEpisode
	if err := dbc.First(&podcast, podcastID).Error; err != nil {
		return nil, fmt.Errorf("find podcast: %w", err)
	}
	return &podcast, nil
}

func streamUpdateStats(dbc *db.DB, userID, albumID int, playTime time.Time) error {
	play := db.Play{
		AlbumID: albumID,
		UserID:  userID,
	}
	err := dbc.
		Where(play).
		First(&play).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find stat: %w", err)
	}

	play.Count++ // for getAlbumList?type=frequent
	if playTime.After(play.Time) {
		play.Time = playTime // for getAlbumList?type=recent
	}
	if err := dbc.Save(&play).Error; err != nil {
		return fmt.Errorf("save stat: %w", err)
	}
	return nil
}

const (
	coverDefaultSize = 600
	coverCacheFormat = "png"
)

var (
	errCoverNotFound = errors.New("could not find a cover with that id")
	errCoverEmpty    = errors.New("no cover found for that folder")
)

func coverGetPath(dbc *db.DB, podcastPath string, id specid.ID) (string, error) {
	switch id.Type {
	case specid.Album:
		return coverGetPathAlbum(dbc, id.Value)
	case specid.Artist:
		return coverGetPathArtist(dbc, id.Value)
	case specid.Podcast:
		return coverGetPathPodcast(dbc, podcastPath, id.Value)
	case specid.PodcastEpisode:
		return coverGetPathPodcastEpisode(dbc, podcastPath, id.Value)
	default:
		return "", errCoverNotFound
	}
}

func coverGetPathAlbum(dbc *db.DB, id int) (string, error) {
	folder := &db.Album{}
	err := dbc.DB.
		Select("id, root_dir, left_path, right_path, cover").
		First(folder, id).
		Error
	if err != nil {
		return "", fmt.Errorf("select album: %w", err)
	}
	if folder.Cover == "" {
		return "", errCoverEmpty
	}
	return path.Join(
		folder.RootDir,
		folder.LeftPath,
		folder.RightPath,
		folder.Cover,
	), nil
}

func coverGetPathArtist(dbc *db.DB, id int) (string, error) {
	folder := &db.Album{}
	err := dbc.DB.
		Select("parent.id, parent.root_dir, parent.left_path, parent.right_path, parent.cover").
		Joins("JOIN albums parent ON parent.id=albums.parent_id").
		Where("albums.tag_artist_id=?", id).
		Find(folder).
		Error
	if err != nil {
		return "", fmt.Errorf("select guessed artist folder: %w", err)
	}
	if folder.Cover == "" {
		return "", errCoverEmpty
	}
	return path.Join(
		folder.RootDir,
		folder.LeftPath,
		folder.RightPath,
		folder.Cover,
	), nil
}

func coverGetPathPodcast(dbc *db.DB, podcastPath string, id int) (string, error) {
	podcast := &db.Podcast{}
	err := dbc.
		First(podcast, id).
		Error
	if err != nil {
		return "", fmt.Errorf("select podcast: %w", err)
	}
	if podcast.ImagePath == "" {
		return "", errCoverEmpty
	}
	return path.Join(podcastPath, podcast.ImagePath), nil
}

func coverGetPathPodcastEpisode(dbc *db.DB, podcastPath string, id int) (string, error) {
	episode := &db.PodcastEpisode{}
	err := dbc.
		First(episode, id).
		Error
	if err != nil {
		return "", fmt.Errorf("select episode: %w", err)
	}
	podcast := &db.Podcast{}
	err = dbc.
		First(podcast, episode.PodcastID).
		Error
	if err != nil {
		return "", fmt.Errorf("select podcast: %w", err)
	}
	if podcast.ImagePath == "" {
		return "", errCoverEmpty
	}
	return path.Join(podcastPath, podcast.ImagePath), nil
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
		coverPath, err := coverGetPath(c.DB, c.PodcastsPath, id)
		if err != nil {
			return spec.NewError(10, "couldn't find cover `%s`: %v", id, err)
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
	var file db.AudioFile
	var audioPath string
	switch id.Type {
	case specid.Track:
		track, err := streamGetTrack(c.DB, id.Value)
		if err != nil {
			return spec.NewError(70, "track with id `%s` was not found", id)
		}
		file = track
		audioPath = path.Join(track.AbsPath())
	case specid.PodcastEpisode:
		podcast, err := streamGetPodcast(c.DB, id.Value)
		if err != nil {
			return spec.NewError(70, "podcast with id `%s` was not found", id)
		}
		file = podcast
		audioPath = path.Join(c.PodcastsPath, podcast.Path)
	default:
		return spec.NewError(70, "media type of `%s` was not found", id.Type)
	}

	user := r.Context().Value(CtxUser).(*db.User)
	if track, ok := file.(*db.Track); ok && track.Album != nil {
		defer func() {
			if err := streamUpdateStats(c.DB, user.ID, track.Album.ID, time.Now()); err != nil {
				log.Printf("error updating status: %v", err)
			}
		}()
	}

	if format, _ := params.Get("format"); format == "raw" {
		http.ServeFile(w, r, audioPath)
		return nil
	}

	pref, err := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(0, "couldn't find transcode preference: %v", err)
	}
	if pref == nil {
		http.ServeFile(w, r, audioPath)
		return nil
	}

	profile, ok := transcode.UserProfiles[pref.Profile]
	if !ok {
		return spec.NewError(0, "unknown transcode user profile %q", pref.Profile)
	}
	if max, _ := params.GetInt("maxBitRate"); max > 0 && int(profile.BitRate()) > max {
		profile = transcode.WithBitrate(profile, transcode.BitRate(max))
	}

	log.Printf("trancoding to %q with max bitrate %dk", profile.MIME(), profile.BitRate())

	w.Header().Set("Content-Type", profile.MIME())
	if err := c.Transcoder.Transcode(r.Context(), profile, audioPath, w); err != nil {
		return spec.NewError(0, "error transcoding: %v", err)
	}

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}
