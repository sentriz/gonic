package spec

import (
	"path"

	"senan.xyz/g/gonic/model"
)

func NewAlbumByTags(a *model.Album, artist *model.Artist) *Album {
	ret := &Album{
		Created: a.ModifiedAt,
		ID:      a.ID,
		Name:    a.TagTitle,
	}
	if a.Cover != "" {
		ret.CoverID = a.ID
	}
	if artist != nil {
		ret.Artist = artist.Name
		ret.ArtistID = artist.ID
	}
	return ret
}

func NewTrackByTags(t *model.Track, album *model.Album) *TrackChild {
	ret := &TrackChild{
		ID:          t.ID,
		ContentType: t.MIME(),
		Suffix:      t.Ext(),
		ParentID:    t.AlbumID,
		CreatedAt:   t.CreatedAt,
		Size:        t.Size,
		Title:       t.TagTitle,
		Artist:      t.TagTrackArtist,
		TrackNumber: t.TagTrackNumber,
		DiscNumber:  t.TagDiscNumber,
		Path: path.Join(
			album.LeftPath,
			album.RightPath,
			t.Filename,
		),
		Album:    album.TagTitle,
		AlbumID:  album.ID,
		Duration: t.Length,
		Bitrate:  t.Bitrate,
		Type:     "music",
	}
	if album.Cover != "" {
		ret.CoverID = album.ID
	}
	if album.TagArtist != nil {
		ret.ArtistID = album.TagArtist.ID
	}
	return ret
}

func NewArtistByTags(a *model.Artist) *Artist {
	return &Artist{
		ID:         a.ID,
		Name:       a.Name,
		AlbumCount: a.AlbumCount,
	}
}
