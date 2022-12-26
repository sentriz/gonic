package spec

import (
	"path"
	"strings"

	"go.senan.xyz/gonic/db"
)

func NewAlbumByTags(a *db.Album, artist *db.Artist) *Album {
	ret := &Album{
		Created:       a.CreatedAt,
		ID:            a.SID(),
		Name:          a.TagTitle,
		Year:          a.TagYear,
		TrackCount:    a.ChildCount,
		Genre:         strings.Join(a.GenreStrings(), ", "),
		Duration:      a.Duration,
		AverageRating: formatRating(a.AverageRating),
	}
	if a.Cover != "" {
		ret.CoverID = a.SID()
	}
	if a.AlbumStar != nil {
		ret.Starred = &a.AlbumStar.StarDate
	}
	if a.AlbumRating != nil {
		ret.UserRating = a.AlbumRating.Rating
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
		Suffix:      formatExt(t.Ext()),
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
		Album:         album.TagTitle,
		AlbumID:       album.SID(),
		Genre:         strings.Join(t.GenreStrings(), ", "),
		Duration:      t.Length,
		Bitrate:       t.Bitrate,
		Type:          "music",
		Year:          album.TagYear,
		AverageRating: formatRating(t.AverageRating),
	}
	if album.Cover != "" {
		ret.CoverID = album.SID()
	}
	if t.TrackStar != nil {
		ret.Starred = &t.TrackStar.StarDate
	}
	if t.TrackRating != nil {
		ret.UserRating = t.TrackRating.Rating
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

func NewArtistByTags(a *db.Artist) *Artist {
	r := &Artist{
		ID:            a.SID(),
		Name:          a.Name,
		AlbumCount:    a.AlbumCount,
		AverageRating: formatRating(a.AverageRating),
	}
	if a.Cover != "" {
		r.CoverID = a.SID()
	}
	if a.ArtistStar != nil {
		r.Starred = &a.ArtistStar.StarDate
	}
	if a.ArtistRating != nil {
		r.UserRating = a.ArtistRating.Rating
	}
	return r
}

func NewGenre(g *db.Genre) *Genre {
	return &Genre{
		Name:       g.Name,
		AlbumCount: g.AlbumCount,
		SongCount:  g.TrackCount,
	}
}
