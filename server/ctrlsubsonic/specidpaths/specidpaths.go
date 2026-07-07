package specidpaths

import (
	"errors"
	"path/filepath"
	"strings"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/fileutil"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

var (
	ErrNotAbs   = errors.New("not abs")
	ErrNotFound = errors.New("not found")
)

type Result interface {
	SID() *specid.ID
	AbsPath() string
}

// LocateMedia resolves an opensubsonic mediaType + id pair (as used by the transcoding and playbackReport
// extensions) to its db.AudioFile. only audio media -- songs and podcast episodes -- is supported.
func LocateMedia(dbc *db.DB, mediaType string, id specid.ID) (db.AudioFile, error) {
	switch {
	case mediaType == "song" && id.Type == specid.Track:
	case mediaType == "podcast" && id.Type == specid.PodcastEpisode:
	default:
		return nil, ErrNotFound
	}
	file, err := Locate(dbc, id)
	if err != nil {
		return nil, err
	}
	audio, ok := file.(db.AudioFile)
	if !ok || audio.AbsPath() == "" {
		return nil, ErrNotFound
	}
	return audio, nil
}

// Locate maps a specid to its location on the filesystem
func Locate(dbc *db.DB, id specid.ID) (Result, error) {
	switch id.Type {
	case specid.Track:
		var track db.Track
		return &track, dbc.Preload("Album").Where("id=?", id.Value).Find(&track).Error
	case specid.PodcastEpisode:
		var pe db.PodcastEpisode
		return &pe, dbc.Preload("Podcast").Where("id=? AND status=?", id.Value, db.PodcastEpisodeStatusCompleted).Find(&pe).Error
	case specid.InternetRadioStation:
		var irs db.InternetRadioStation
		return &irs, dbc.Where("id=?", id.Value).Find(&irs).Error
	default:
		return nil, ErrNotFound
	}
}

// Lookup maps a location on the filesystem to a specid
func Lookup(dbc *db.DB, musicPaths []string, podcastsPath string, path string) (*specid.ID, error) {
	if !strings.HasPrefix(path, "http") && !filepath.IsAbs(path) {
		return nil, ErrNotAbs
	}

	if strings.HasPrefix(path, podcastsPath) {
		podcastPath, episodeFilename := filepath.Split(path)

		q := dbc.
			Select("podcast_episodes.id").
			Model(db.PodcastEpisode{}).
			Joins(`JOIN podcasts ON podcasts.id=podcast_episodes.podcast_id`).
			Where(`podcasts.root_dir=? AND podcast_episodes.filename=?`, filepath.Clean(podcastPath), filepath.Clean(episodeFilename))

		var id int
		if err := q.Row().Scan(&id); err != nil {
			return nil, ErrNotFound
		}
		return &specid.ID{Type: specid.PodcastEpisode, Value: id}, nil
	}

	// probably internet radio
	if strings.HasPrefix(path, "http") {
		q := dbc.
			Select("internet_radio_stations.id").
			Model(db.InternetRadioStation{}).
			Where("stream_url=?", path)

		var id int
		if err := q.Row().Scan(&id); err != nil {
			return nil, ErrNotFound
		}
		return &specid.ID{Type: specid.InternetRadioStation, Value: id}, nil
	}

	var musicPath string
	for _, mp := range musicPaths {
		if fileutil.HasPrefix(path, mp) {
			musicPath = mp
			break
		}
	}
	if musicPath == "" {
		return nil, ErrNotFound
	}

	relPath, _ := filepath.Rel(musicPath, path)
	relDir, filename := filepath.Split(relPath)
	leftPath, rightPath := filepath.Split(filepath.Clean(relDir))

	q := dbc.
		Select("tracks.id").
		Model(db.Track{}).
		Where(`albums.root_dir=? AND albums.left_path=? AND albums.right_path=? AND tracks.filename=?`, musicPath, leftPath, rightPath, filename).
		Joins(`JOIN albums ON tracks.album_id=albums.id`)

	var id int
	if err := q.Row().Scan(&id); err != nil {
		return nil, ErrNotFound
	}
	return &specid.ID{Type: specid.Track, Value: id}, nil
}
