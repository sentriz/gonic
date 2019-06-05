package handler

import (
	"path"

	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/server/subsonic"
)

func newAlbumByTags(a *model.Album, artist *model.Artist) *subsonic.Album {
	ret := &subsonic.Album{
		CoverID: a.ID,
		Created: a.CreatedAt,
		ID:      a.ID,
		Name:    a.TagTitle,
	}
	if artist != nil {
		ret.Artist = artist.Name
		ret.ArtistID = artist.ID
	}
	return ret
}

func newTrackByTags(t *model.Track, album *model.Album) *subsonic.TrackChild {
	return &subsonic.TrackChild{
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

func newArtistByTags(a *model.Artist) *subsonic.Artist {
	return &subsonic.Artist{
		ID:   a.ID,
		Name: a.Name,
	}
}
