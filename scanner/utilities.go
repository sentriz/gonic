package scanner

import (
	"os"

	"github.com/dhowden/tag"
	"github.com/pkg/errors"
)

var coverFilenames = map[string]struct{}{
	"cover.png":   {},
	"cover.jpg":   {},
	"cover.jpeg":  {},
	"folder.png":  {},
	"folder.jpg":  {},
	"folder.jpeg": {},
	"album.png":   {},
	"album.jpg":   {},
	"album.jpeg":  {},
	"front.png":   {},
	"front.jpg":   {},
	"front.jpeg":  {},
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
