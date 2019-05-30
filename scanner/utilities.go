package scanner

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dhowden/tag"
	"github.com/pkg/errors"
)

var trackExtensions = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/x-flac",
	"aac":  "audio/x-aac",
	"m4a":  "audio/m4a",
	"ogg":  "audio/ogg",
}

func isTrack(fullPath string) (string, string, bool) {
	ext := filepath.Ext(fullPath)[1:]
	mine, ok := trackExtensions[ext]
	if !ok {
		return "", "", false
	}
	return mine, ext, true
}

var coverFilenames = map[string]struct{}{
	"cover.png":   struct{}{},
	"cover.jpg":   struct{}{},
	"cover.jpeg":  struct{}{},
	"folder.png":  struct{}{},
	"folder.jpg":  struct{}{},
	"folder.jpeg": struct{}{},
	"album.png":   struct{}{},
	"album.jpg":   struct{}{},
	"album.jpeg":  struct{}{},
	"front.png":   struct{}{},
	"front.jpg":   struct{}{},
	"front.jpeg":  struct{}{},
}

func isCover(fullPath string) bool {
	_, filename := path.Split(fullPath)
	_, ok := coverFilenames[strings.ToLower(filename)]
	return ok
}

func readTags(path string) (tag.Metadata, error) {
	trackData, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading track from disk")
	}
	defer trackData.Close()
	tags, err := tag.ReadFrom(trackData)
	if err != nil {
		return nil, errors.Wrap(err, "reading tags from track")
	}
	return tags, nil
}
