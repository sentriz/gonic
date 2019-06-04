package scanner

import (
	"os"

	"github.com/dhowden/tag"
	"github.com/pkg/errors"
)

var mimeTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/x-flac",
	"aac":  "audio/x-aac",
	"m4a":  "audio/m4a",
	"ogg":  "audio/ogg",
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
