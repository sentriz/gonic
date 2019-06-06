package scanner

import (
	"os"
	"strconv"
	"strings"

	"github.com/mdlayher/taggolib"
	"github.com/pkg/errors"
)

type tags struct {
	taggolib.Parser
}

func (t *tags) Year() int {
	// for this one there could be multiple tags and a
	// date separator. so do string(2019-6-6) -> int(2019)
	// for the two options and pick the first
	for _, name := range []string{"DATE", "YEAR"} {
		tag := t.Tag(name)
		if tag == "" {
			continue
		}
		parts := strings.Split(tag, "-")
		year, err := strconv.Atoi(parts[0])
		if err != nil {
			continue
		}
		return year
	}
	return 0
}

func (t *tags) DurationSecs() int {
	duration := int(t.Duration() / 1e9)
	if duration == 0 {
		return -1
	}
	return duration
}

func readTags(path string) (*tags, error) {
	track, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading disk")
	}
	defer track.Close()
	parser, err := taggolib.New(track)
	if err != nil {
		return nil, errors.Wrap(err, "parsing")
	}
	newTags := &tags{parser}
	return newTags, nil
}
