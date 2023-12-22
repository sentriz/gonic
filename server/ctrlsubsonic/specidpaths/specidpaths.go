package specidpaths

import (
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"go.senan.xyz/gonic/db"
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

// Locate maps a specid to its location on the filesystem
func Locate(dbc *db.DB, id specid.ID) (Result, error) {
	switch id.Type {
	case specid.Track:
		var track db.Track
		return &track, dbc.Preload("Album").Where("id=?", id.Value).Find(&track).Error
	case specid.PodcastEpisode:
		var pe db.PodcastEpisode
		return &pe, dbc.Preload("Podcast").Where("id=? AND status=?", id.Value, db.PodcastEpisodeStatusCompleted).Find(&pe).Error
	default:
		return nil, ErrNotFound
	}
}

// Locate maps a location on the filesystem to a specid
func Lookup(dbc *db.DB, musicPaths []string, podcastsPath string, path string) (Result, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrNotAbs
	}

	if strings.HasPrefix(path, podcastsPath) {
		podcastPath, episodeFilename := filepath.Split(path)
		q := dbc.
			Joins(`JOIN podcasts ON podcasts.id=podcast_episodes.podcast_id`).
			Where(`podcasts.root_dir=? AND podcast_episodes.filename=?`, filepath.Clean(podcastPath), filepath.Clean(episodeFilename))

		var pe db.PodcastEpisode
		if err := q.First(&pe).Error; err == nil {
			return &pe, nil
		}
		return nil, ErrNotFound
	}

	var musicPath string
	for _, mp := range musicPaths {
		if strings.HasPrefix(path, mp + '/') {
			musicPath = mp
		}
	}
	if musicPath == "" {
		return nil, ErrNotFound
	}

	relPath, _ := filepath.Rel(musicPath, path)
	relDir, filename := filepath.Split(relPath)
	leftPath, rightPath := filepath.Split(filepath.Clean(relDir))

	q := dbc.
		Where(`albums.root_dir=? AND albums.left_path=? AND albums.right_path=? AND tracks.filename=?`, musicPath, leftPath, rightPath, filename).
		Joins(`JOIN albums ON tracks.album_id=albums.id`).
		Preload("Album")

	var track db.Track
	if err := q.First(&track).Error; err == nil {
		return &track, nil
	}
	return nil, ErrNotFound
}
