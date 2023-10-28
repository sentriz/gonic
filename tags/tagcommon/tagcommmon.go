package tagcommon

import (
	"errors"
)

var ErrUnsupported = errors.New("filetype unsupported")

type Reader interface {
	CanRead(absPath string) bool
	Read(absPath string) (Info, error)
}

type Info interface {
	Title() string
	BrainzID() string
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
	Length() int
	Bitrate() int
	Year() int
}

func MustAlbum(p Info) string {
	if r := p.Album(); r != "" {
		return r
	}
	return "Unknown Album"
}

func MustArtist(p Info) string {
	if r := p.Artist(); r != "" {
		return r
	}
	return "Unknown Artist"
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
	return "Unknown Genre"
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
