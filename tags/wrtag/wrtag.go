package wrtag

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"

	"go.senan.xyz/gonic/tags"
	wrtag "go.senan.xyz/wrtag/tags"
)

type Reader struct{}

func (Reader) CanRead(absPath string) bool {
	return wrtag.CanRead(absPath)
}

func (Reader) Read(absPath string) (tags.Info, error) {
	t, err := wrtag.ReadTags(absPath)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	p, err := wrtag.ReadProperties(absPath)
	if err != nil {
		return nil, fmt.Errorf("read properties: %w", err)
	}
	return &info{t, p}, nil
}

type info struct {
	tags  wrtag.Tags
	props wrtag.Properties
}

func (i *info) Title() string          { return i.tags.Get(wrtag.Title) }
func (i *info) BrainzID() string       { return i.tags.Get(wrtag.MusicBrainzRecordingID) }
func (i *info) Artist() string         { return i.tags.Get(wrtag.Artist) }
func (i *info) Artists() []string      { return i.tags.Values(wrtag.Artists) }
func (i *info) Album() string          { return i.tags.Get(wrtag.Album) }
func (i *info) AlbumArtist() string    { return i.tags.Get(wrtag.AlbumArtist) }
func (i *info) AlbumArtists() []string { return i.tags.Values(wrtag.AlbumArtists) }
func (i *info) AlbumBrainzID() string  { return i.tags.Get(wrtag.MusicBrainzReleaseID) }
func (i *info) Genre() string          { return i.tags.Get(wrtag.Genre) }
func (i *info) Genres() []string       { return i.tags.Values(wrtag.Genres) }
func (i *info) TrackNumber() int       { return intSep(i.tags.Get(wrtag.TrackNumber)) }
func (i *info) DiscNumber() int        { return intSep(i.tags.Get(wrtag.DiscNumber)) }
func (i *info) Year() int {
	return intSep(cmp.Or(i.tags.Get(wrtag.OriginalDate), i.tags.Get(wrtag.Date)))
}
func (i *info) Lyrics() string { return i.tags.Get(wrtag.Lyrics) }

func (i *info) Compilation() bool   { return bl(i.tags.Get(wrtag.Compilation)) }
func (i *info) ReleaseType() string { return i.tags.Get(wrtag.ReleaseType) }

func (i *info) ReplayGainTrackGain() float32 { return dB(i.tags.Get(wrtag.ReplayGainTrackGain)) }
func (i *info) ReplayGainTrackPeak() float32 { return flt(i.tags.Get(wrtag.ReplayGainTrackPeak)) }
func (i *info) ReplayGainAlbumGain() float32 { return dB(i.tags.Get(wrtag.ReplayGainAlbumGain)) }
func (i *info) ReplayGainAlbumPeak() float32 { return flt(i.tags.Get(wrtag.ReplayGainAlbumPeak)) }

func (i *info) Length() int  { return int(i.props.Length.Seconds()) }
func (i *info) Bitrate() int { return int(i.props.Bitrate) }

func flt(in string) float32 {
	f, _ := strconv.ParseFloat(in, 32)
	return float32(f)
}

func bl(in string) bool {
	b, _ := strconv.ParseBool(in)
	return b
}

func dB(in string) float32 {
	in = strings.ToLower(in)
	in = strings.TrimSuffix(in, " db")
	in = strings.TrimSuffix(in, "db")
	return flt(in)
}

func intSep(in string) int {
	start, _, _ := strings.Cut(in, "/")
	out, _ := strconv.Atoi(start)
	return out
}
