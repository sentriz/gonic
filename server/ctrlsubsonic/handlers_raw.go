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

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
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

func streamGetPodcast(dbc *db.DB, podcastID int) (*db.PodcastEpisode, error) {
	podcast := db.PodcastEpisode{}
	err := dbc.First(&podcast, podcastID).Error
	return &podcast, err
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

func coverGetPath(dbc *db.DB, musicPath, podcastPath string, id specid.ID) (string, error) {
	switch id.Type {
	case specid.Album:
		return coverGetPathAlbum(dbc, musicPath, id.Value)
	case specid.Podcast:
		return coverGetPathPodcast(dbc, podcastPath, id.Value)
	case specid.PodcastEpisode:
		return coverGetPathPodcastEpisode(dbc, podcastPath, id.Value)
	default:
		return "", errCoverNotFound
	}
}

func coverGetPathAlbum(dbc *db.DB, musicPath string, id int) (string, error) {
	folder := &db.Album{}
	err := dbc.DB.
		Select("id, left_path, right_path, cover").
		First(folder, id).
		Error
	if err != nil {
		return "", fmt.Errorf("select album: %w", err)
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
		coverPath, err := coverGetPath(c.DB, c.MusicPath, c.Podcasts.PodcastBasePath, id)
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
	var audioFile db.AudioFile
	var audioPath string
	switch id.Type {
	case specid.Track:
		track, _ := streamGetTrack(c.DB, id.Value)
		audioFile = track
		audioPath = path.Join(c.MusicPath, track.RelPath())
		if err != nil {
			return spec.NewError(70, "track with id `%s` was not found", id)
		}
	case specid.PodcastEpisode:
		podcast, err := streamGetPodcast(c.DB, id.Value)
		audioFile = podcast
		audioPath = path.Join(c.Podcasts.PodcastBasePath, podcast.Path)
		if err != nil {
			return spec.NewError(70, "podcast with id `%s` was not found", id)
		}
	}

	if err != nil && id.Type != specid.Podcast {
		return spec.NewError(70, "media with id `%d` was not found", id.Value)
	}
	user := r.Context().Value(CtxUser).(*db.User)
	if track, ok := audioFile.(*db.Track); ok {
		defer streamUpdateStats(c.DB, user.ID, track.Album.ID)
	}
	pref := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	//
	onInvalidProfile := func() error {
		log.Printf("serving raw `%s`\n", audioFile.AudioFilename())
		w.Header().Set("Content-Type", audioFile.MIME())
		http.ServeFile(w, r, audioPath)
		return nil
	}
	onCacheHit := func(profile encode.Profile, path string) error {
		log.Printf("serving transcode `%s`: cache [%s/%dk] hit!\n",
			audioFile.AudioFilename(), profile.Format, profile.Bitrate)
		http.ServeFile(w, r, path)
		return nil
	}
	onCacheMiss := func(profile encode.Profile) (io.Writer, error) {
		log.Printf("serving transcode `%s`: cache [%s/%dk] miss!\n",
			audioFile.AudioFilename(), profile.Format, profile.Bitrate)
		return w, nil
	}
	encodeOptions := encode.Options{
		TrackPath:        audioPath,
		TrackBitrate:     audioFile.AudioBitrate(),
		CachePath:        c.CachePath,
		ProfileName:      pref.Profile,
		PreferredBitrate: params.GetOrInt("maxBitRate", 0),
		OnInvalidProfile: onInvalidProfile,
		OnCacheHit:       onCacheHit,
		OnCacheMiss:      onCacheMiss,
	}
	if err := encode.Encode(encodeOptions); err != nil {
		log.Printf("serving transcode `%s`: error: %v\n", audioFile.AudioFilename(), err)
	}
	return nil
}

func (c *Controller) ServeDownload(w http.ResponseWriter, r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	var filePath string
	var audioFile db.AudioFile
	switch id.Type {
	case specid.Track:
		track, _ := streamGetTrack(c.DB, id.Value)
		audioFile = track
		filePath = path.Join(c.MusicPath, track.RelPath())
		if err != nil {
			return spec.NewError(70, "track with id `%s` was not found", id)
		}
	case specid.PodcastEpisode:
		podcast, err := streamGetPodcast(c.DB, id.Value)
		audioFile = podcast
		filePath = path.Join(c.Podcasts.PodcastBasePath, podcast.Path)
		if err != nil {
			return spec.NewError(70, "podcast with id `%s` was not found", id)
		}
	}
	log.Printf("serving raw `%s`\n", audioFile.AudioFilename())
	w.Header().Set("Content-Type", audioFile.MIME())
	http.ServeFile(w, r, filePath)
	return nil
}
