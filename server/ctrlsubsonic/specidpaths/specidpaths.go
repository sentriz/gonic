package specidpaths

import (
	"errors"
	"path/filepath"
	"strings"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

var ErrNotAbs = errors.New("not abs")
var ErrNotFound = errors.New("not found")
var ErrPodcastStatus = errors.New("wrong podcast status is ")

type Result interface {
	SID() *specid.ID
	AbsPath() string
}

// Locate maps a specid to its location on the filesystem
func Locate(dbc *db.DB, podcastsPath string, id specid.ID) (Result, error) {
	switch id.Type {
	case specid.Track:
		var track db.Track
		if err := dbc.Preload("Album").Where("id=?", id.Value).Find(&track).Error; err == nil {
			return &track, nil
		}
	case specid.PodcastEpisode:
		var pe db.PodcastEpisode
		if err := dbc.Where("id=? AND status=?", id.Value, db.PodcastEpisodeStatusCompleted).Find(&pe).Error; err == nil {
			pe.AbsP = filepath.Join(podcastsPath, pe.Path)
			return &pe, err
		}
	}
	return nil, ErrNotFound
}

// Locate maps a location on the filesystem to a specid
func Lookup(dbc *db.DB, musicPaths []string, podcastsPath string, path string) (Result, error) {
	if !filepath.IsAbs(path) {
		return nil, ErrNotAbs
	}

	if strings.HasPrefix(path, podcastsPath) {
		path, _ = filepath.Rel(podcastsPath, path)
		var pe db.PodcastEpisode
		if err := dbc.Where(`path=?`, path).First(&pe).Error; err == nil {
			return &pe, nil
		}
		return nil, ErrNotFound
	}

	var musicPath string
	for _, mp := range musicPaths {
		if strings.HasPrefix(path, mp) {
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
