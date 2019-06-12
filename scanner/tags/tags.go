package tags

import (
	"github.com/nicksellen/audiotags"
	"github.com/pkg/errors"
)

type Tags struct {
	raw   map[string]string
	props *audiotags.AudioProperties
}

func New(path string) (*Tags, error) {
	raw, props, err := audiotags.Read(path)
	if err != nil {
		return nil, errors.Wrap(err, "audiotags module")
	}
	return &Tags{
		raw:   raw,
		props: props,
	}, nil
}

func (t *Tags) firstTag(keys ...string) string {
	for _, key := range keys {
		if val, ok := t.raw[key]; ok {
			return val
		}
	}
	return ""
}

func (t *Tags) Title() string       { return t.firstTag("title") }
func (t *Tags) Artist() string      { return t.firstTag("artist") }
func (t *Tags) Album() string       { return t.firstTag("album") }
func (t *Tags) AlbumArtist() string { return t.firstTag("albumartist", "album artist") }
func (t *Tags) Year() int           { return intSep(t.firstTag("date", "year"), "-") } // eg. 2019-6-11
func (t *Tags) TrackNumber() int    { return intSep(t.firstTag("tracknumber"), "/") }  // eg. 5/12
func (t *Tags) DiscNumber() int     { return intSep(t.firstTag("discnumber"), "/") }   // eg. 1/2
func (t *Tags) Length() int         { return t.props.Length }
func (t *Tags) Bitrate() int        { return t.props.Bitrate }
