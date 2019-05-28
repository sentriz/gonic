package handler

import (
	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func makeAlbumFromAlbum(a *model.Album, artist *model.Artist) *subsonic.Album {
	return &subsonic.Album{
		ID:       a.ID,
		Name:     a.Title,
		Created:  a.CreatedAt,
		CoverID:  a.CoverID,
		Artist:   artist.Name,
		ArtistID: artist.ID,
	}
}

func makeTrackFromTrack(t *model.Track, album *model.Album) *subsonic.Track {
	return &subsonic.Track{
		ID:          t.ID,
		Title:       t.Title,
		Artist:      t.TrackArtist,
		TrackNumber: t.TrackNumber,
		ContentType: t.ContentType,
		Path:        t.Path,
		Suffix:      t.Suffix,
		CreatedAt:   t.CreatedAt,
		Size:        t.Size,
		Album:       album.Title,
		AlbumID:     album.ID,
		ArtistID:    album.Artist.ID,
		CoverID:     album.CoverID,
		Type:        "music",
	}
}

func makeArtistFromArtist(a *model.Artist) *subsonic.Artist {
	return &subsonic.Artist{
		ID:   a.ID,
		Name: a.Name,
	}
}
