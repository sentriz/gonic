package taglib

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sentriz/audiotags"
	"go.senan.xyz/gonic/tags/tagcommon"
)

type TagLib struct{}

func (TagLib) CanRead(absPath string) bool {
	switch ext := strings.ToLower(filepath.Ext(absPath)); ext {
	case ".mp3", ".flac", ".aac", ".m4a", ".m4b", ".ogg", ".opus", ".wma", ".wav", ".wv":
		return true
	}
	return false
}

func (TagLib) Read(absPath string) (tagcommon.Info, error) {
	f, err := audiotags.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	props := f.ReadAudioProperties()
	raw := f.ReadTags()
	return &info{raw, props, absPath}, nil
}

type info struct {
	raw     map[string][]string
	props   *audiotags.AudioProperties
	abspath string
}

// https://picard-docs.musicbrainz.org/downloads/MusicBrainz_Picard_Tag_Map.html

func (i *info) Title() string          { return first(find(i.raw, "title")) }
func (i *info) BrainzID() string       { return first(find(i.raw, "musicbrainz_trackid")) } // musicbrainz recording ID
func (i *info) Artist() string         { return first(find(i.raw, "artist")) }
func (i *info) Artists() []string      { return find(i.raw, "artists") }
func (i *info) Album() string          { return first(find(i.raw, "album")) }
func (i *info) AlbumArtist() string    { return first(find(i.raw, "albumartist", "album artist")) }
func (i *info) AlbumArtists() []string { return find(i.raw, "albumartists", "album_artists") }
func (i *info) AlbumBrainzID() string  { return first(find(i.raw, "musicbrainz_albumid")) } // musicbrainz release ID
func (i *info) Genre() string          { return first(find(i.raw, "genre")) }
func (i *info) Genres() []string       { return find(i.raw, "genres") }
func (i *info) TrackNumber() int       { return intSep("/", first(find(i.raw, "tracknumber"))) }                  // eg. 5/12
func (i *info) DiscNumber() int        { return intSep("/", first(find(i.raw, "discnumber"))) }                   // eg. 1/2
func (i *info) Year() int              { return intSep("-", first(find(i.raw, "originaldate", "date", "year"))) } // eg. 2023-12-01

func (i *info) ReplayGainTrackGain() float32 { return dB(first(find(i.raw, "replaygain_track_gain"))) }
func (i *info) ReplayGainTrackPeak() float32 { return flt(first(find(i.raw, "replaygain_track_peak"))) }
func (i *info) ReplayGainAlbumGain() float32 { return dB(first(find(i.raw, "replaygain_album_gain"))) }
func (i *info) ReplayGainAlbumPeak() float32 { return flt(first(find(i.raw, "replaygain_album_peak"))) }

func (i *info) Length() int  { return i.props.Length }
func (i *info) Bitrate() int { return i.props.Bitrate }

func (i *info) AbsPath() string { return i.abspath }

func first[T comparable](is []T) T {
	var z T
	for _, i := range is {
		if i != z {
			return i
		}
	}
	return z
}

func find(m map[string][]string, keys ...string) []string {
	for _, k := range keys {
		if r := filterStr(m[k]); len(r) > 0 {
			return r
		}
	}
	return nil
}

func filterStr(ss []string) []string {
	var r []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			r = append(r, s)
		}
	}
	return r
}

func flt(in string) float32 {
	f, _ := strconv.ParseFloat(in, 32)
	return float32(f)
}
func dB(in string) float32 {
	in = strings.ToLower(in)
	in = strings.TrimSuffix(in, " db")
	in = strings.TrimSuffix(in, "db")
	return flt(in)
}

func intSep(sep, in string) int {
	start, _, _ := strings.Cut(in, sep)
	out, _ := strconv.Atoi(start)
	return out
}
