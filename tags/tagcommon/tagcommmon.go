package tagcommon

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/disintegration/imaging"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	CanRead(absPath string) bool
	Read(absPath string) (Info, error)
}

type Info interface {
	Title() string
	BrainzID() string // musicbrainz recording ID
	Artist() string
	Artists() []string
	Album() string
	AlbumArtist() string
	AlbumArtists() []string
	AlbumBrainzID() string
	Genre() string
	Genres() []string
	TrackNumber() int
	DiscNumber() int
	Year() int
	EmbeddedCover(string) io.Reader

	ReplayGainTrackGain() float32
	ReplayGainTrackPeak() float32
	ReplayGainAlbumGain() float32
	ReplayGainAlbumPeak() float32

	Length() int
	Bitrate() int
}

const (
	FallbackAlbum  = "Unknown Album"
	FallbackArtist = "Unknown Artist"
	FallbackGenre  = "Unknown Genre"

	CoverDefaultSize = 600
)

func CachePath(cacheDir, id string, size int) string {
	return filepath.Join(cacheDir, fmt.Sprintf("%s-%d.png", id, size))
}

func CoverScaleAndSave(reader io.Reader, cachePath string, size int) error {
	src, err := imaging.Decode(reader)
	if err != nil {
		return fmt.Errorf("resizing: %w", err)
	}
	width := size
	if width > src.Bounds().Dx() {
		// don't upscale images
		width = src.Bounds().Dx()
	}
	if err := imaging.Save(imaging.Resize(src, width, 0, imaging.Lanczos), cachePath); err != nil {
		return fmt.Errorf("caching %q: %w", cachePath, err)
	}
	return nil
}

func MustAlbum(p Info) string {
	if r := p.Album(); r != "" {
		return r
	}
	return FallbackAlbum
}

func MustArtist(p Info) string {
	if r := p.Artist(); r != "" {
		return r
	}
	return FallbackArtist
}

func MustArtists(p Info) []string {
	if r := p.Artists(); len(r) > 0 {
		return r
	}
	return []string{MustArtist(p)}
}

func MustAlbumArtist(p Info) string {
	if r := p.AlbumArtist(); r != "" {
		return r
	}
	return MustArtist(p)
}

func MustAlbumArtists(p Info) []string {
	if r := p.AlbumArtists(); len(r) > 0 {
		return r
	}
	return []string{MustAlbumArtist(p)}
}

func MustGenre(p Info) string {
	if r := p.Genre(); r != "" {
		return r
	}
	return FallbackGenre
}

func MustGenres(p Info) []string {
	if r := p.Genres(); len(r) > 0 {
		return r
	}
	return []string{MustGenre(p)}
}

type ChainReader []Reader

func (cr ChainReader) CanRead(absPath string) bool {
	for _, reader := range cr {
		if reader.CanRead(absPath) {
			return true
		}
	}
	return false
}

func (cr ChainReader) Read(absPath string) (Info, error) {
	for _, reader := range cr {
		if reader.CanRead(absPath) {
			return reader.Read(absPath)
		}
	}
	return nil, ErrUnsupported
}
