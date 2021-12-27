package spec

import (
	"path"
	"strings"

	"go.senan.xyz/gonic/server/db"
)

func NewAlbumByTags(a *db.Album, artist *db.Artist) *Album {
	ret := &Album{
		Created:    a.CreatedAt,
		ID:         a.SID(),
		Name:       a.TagTitle,
		Year:       a.TagYear,
		TrackCount: a.ChildCount,
		Genre:      strings.Join(a.GenreStrings(), ", "),
		Duration:   a.Duration,
	}
	if a.Cover != "" {
		ret.CoverID = a.SID()
	}
	if artist != nil {
		ret.Artist = artist.Name
		ret.ArtistID = artist.SID()
	}
	return ret
}

func NewTrackByTags(t *db.Track, album *db.Album) *TrackChild {
	ret := &TrackChild{
		ID:          t.SID(),
		ContentType: t.MIME(),
		Suffix:      t.Ext(),
		ParentID:    t.AlbumSID(),
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
		AlbumID:  album.SID(),
		Genre:    strings.Join(t.GenreStrings(), ", "),
		Duration: t.Length,
		Bitrate:  t.Bitrate,
		Type:     "music",
		Year:     album.TagYear,
	}
	if album.Cover != "" {
		ret.CoverID = album.SID()
	}
	if album.TagArtist != nil {
		ret.ArtistID = album.TagArtist.SID()
	}
	// replace tags that we're present
	if ret.Title == "" {
		ret.Title = "<title>"
	}
	if ret.Artist == "" {
		ret.Artist = "<artist>"
	}
	if ret.Album == "" {
		ret.Album = "<album>"
	}
	return ret
}

func NewArtistByTags(a *db.Artist, dbc *db.DB) *Artist {
	// attempt to fetch a cover if possible
	guessedArtistFolder := &db.Album{}
	_ = dbc.
		Select("parent.*").
		Joins("JOIN albums parent ON parent.id=albums.parent_id").
		// Where("albums.tag_artist_id=?", id.Value).
		Where("albums.tag_artist_id=?", a.ID).
		Find(&guessedArtistFolder).
		Error

	if guessedArtistFolder.Cover != "" {
		return &Artist{
			ID:         a.SID(),
			Name:       a.Name,
			AlbumCount: a.AlbumCount,
			CoverID:    a.SID(),
		}
	}

	return &Artist{
		ID:         a.SID(),
		Name:       a.Name,
		AlbumCount: a.AlbumCount,
	}

}

func NewGenre(g *db.Genre) *Genre {
	return &Genre{
		Name:       g.Name,
		AlbumCount: g.AlbumCount,
		SongCount:  g.TrackCount,
	}
}
