package handler

import (
	"path"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func makeAlbumFromAlbum(a *model.Album, artist *model.Artist) *subsonic.Album {
	ret := &subsonic.Album{
		ID:      a.ID,
		Name:    a.TagTitle,
		Created: a.CreatedAt,
		CoverID: a.ID,
	}
	if artist != nil {
		ret.Artist = artist.Name
		ret.ArtistID = artist.ID
	}
	return ret
}

func makeTrackFromTrack(t *model.Track, album *model.Album) *subsonic.Track {
	return &subsonic.Track{
		ID:          t.ID,
		ContentType: t.MIME(),
		Suffix:      t.Ext(),
		ParentID:    t.AlbumID,
		CreatedAt:   t.CreatedAt,
		Size:        t.Size,
		Title:       t.TagTitle,
		Artist:      t.TagTrackArtist,
		TrackNumber: t.TagTrackNumber,
		Path: path.Join(
			album.LeftPath,
			album.RightPath,
			t.Filename,
		),
		Album:    album.TagTitle,
		AlbumID:  album.ID,
		ArtistID: album.TagArtist.ID,
		CoverID:  album.ID,
		Type:     "music",
	}
}

func makeArtistFromArtist(a *model.Artist) *subsonic.Artist {
	return &subsonic.Artist{
		ID:   a.ID,
		Name: a.Name,
	}
}
