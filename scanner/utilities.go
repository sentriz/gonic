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

var coverFilenames = map[string]bool{
	"cover.png":   true,
	"cover.jpg":   true,
	"cover.jpeg":  true,
	"folder.png":  true,
	"folder.jpg":  true,
	"folder.jpeg": true,
	"album.png":   true,
	"album.jpg":   true,
	"album.jpeg":  true,
	"front.png":   true,
	"front.jpg":   true,
	"front.jpeg":  true,
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
