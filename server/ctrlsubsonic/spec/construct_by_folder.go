package spec

import (
	"path/filepath"

	"go.senan.xyz/gonic/db"
)

func NewAlbumByFolder(f *db.Album) *Album {
	a := &Album{
		Artist:        f.Parent.RightPath,
		ID:            f.SID(),
		IsDir:         true,
		ParentID:      f.ParentSID(),
		Album:         f.RightPath,
		Name:          f.RightPath,
		Title:         f.RightPath,
		TrackCount:    f.ChildCount,
		Duration:      f.Duration,
		Created:       f.CreatedAt,
		AverageRating: formatRating(f.AverageRating),
	}
	if f.AlbumStar != nil {
		a.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		a.UserRating = f.AlbumRating.Rating
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	}
	return a
}

func NewTCAlbumByFolder(f *db.Album) *TrackChild {
	trCh := &TrackChild{
		ID:            f.SID(),
		IsDir:         true,
		Title:         f.RightPath,
		ParentID:      f.ParentSID(),
		CreatedAt:     f.CreatedAt,
		AverageRating: formatRating(f.AverageRating),
		Year:          f.TagYear,
	}
	if f.AlbumStar != nil {
		trCh.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		trCh.UserRating = f.AlbumRating.Rating
	}
	if f.Cover != "" {
		trCh.CoverID = f.SID()
	}
	return trCh
}

func NewTCTrackByFolder(t *db.Track, parent *db.Album) *TrackChild {
	trCh := &TrackChild{
		ID:          t.SID(),
		ContentType: t.MIME(),
		Suffix:      formatExt(t.Ext()),
		Size:        t.Size,
		Artist:      t.TagTrackArtist,
		Title:       t.TagTitle,
		TrackNumber: t.TagTrackNumber,
		DiscNumber:  t.TagDiscNumber,
		Path: filepath.Join(
			parent.LeftPath,
			parent.RightPath,
			t.Filename,
		),
		ParentID:      parent.SID(),
		Duration:      t.Length,
		Year:          parent.TagYear,
		Bitrate:       t.Bitrate,
		IsDir:         false,
		Type:          "music",
		MusicBrainzID: t.TagBrainzID,
		CreatedAt:     t.CreatedAt,
		AverageRating: formatRating(t.AverageRating),
	}
	if trCh.Title == "" {
		trCh.Title = t.Filename
	}
	if parent.Cover != "" {
		trCh.CoverID = parent.SID()
	}
	if t.Album != nil {
		trCh.Album = t.Album.RightPath
		trCh.AlbumID = t.Album.SID()
	}
	if t.TrackStar != nil {
		trCh.Starred = &t.TrackStar.StarDate
	}
	if t.TrackRating != nil {
		trCh.UserRating = t.TrackRating.Rating
	}
	if len(t.Genres) > 0 {
		trCh.Genre = t.Genres[0].Name
	}
	for _, g := range t.Genres {
		trCh.Genres = append(trCh.Genres, &GenreRef{Name: g.Name})
	}
	for _, a := range t.Artists {
		if a.Name == t.TagTrackArtist {
			trCh.ArtistID = a.SID()
		}
		trCh.Artists = append(trCh.Artists, &ArtistRef{ID: a.SID(), Name: a.Name})
	}
	if t.ReplayGainTrackGain != 0 || t.ReplayGainAlbumGain != 0 {
		trCh.ReplayGain = &ReplayGain{
			TrackGain: t.ReplayGainTrackGain,
			TrackPeak: t.ReplayGainTrackPeak,
			AlbumGain: t.ReplayGainAlbumGain,
			AlbumPeak: t.ReplayGainAlbumPeak,
		}
	}
	return trCh
}

func NewTCPodcastEpisode(pe *db.PodcastEpisode) *TrackChild {
	trCh := &TrackChild{
		ID:          pe.SID(),
		ContentType: pe.MIME(),
		Suffix:      pe.Ext(),
		Size:        pe.Size,
		Title:       pe.Title,
		ParentID:    pe.SID(),
		Duration:    pe.Length,
		Bitrate:     pe.Bitrate,
		IsDir:       false,
		Type:        "podcastepisode",
		CreatedAt:   pe.CreatedAt,
	}
	if pe.Podcast != nil {
		trCh.ParentID = pe.Podcast.SID()
		trCh.Path = pe.AbsPath()
	}
	return trCh
}

func NewArtistByFolder(f *db.Album) *Artist {
	// the db is structued around "browse by tags", and where
	// an album is also a folder. so we're constructing an artist
	// from an "album" where
	// maybe TODO: rename the Album model to Folder
	a := &Artist{
		ID:            f.SID(),
		Name:          f.RightPath,
		AlbumCount:    f.ChildCount,
		AverageRating: formatRating(f.AverageRating),
	}
	if f.AlbumStar != nil {
		a.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		a.UserRating = f.AlbumRating.Rating
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	}
	return a
}

func NewDirectoryByFolder(f *db.Album, children []*TrackChild) *Directory {
	d := &Directory{
		ID:            f.SID(),
		Name:          f.RightPath,
		Children:      children,
		ParentID:      f.ParentSID(),
		AverageRating: formatRating(f.AverageRating),
	}
	if f.AlbumStar != nil {
		d.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		d.UserRating = f.AlbumRating.Rating
	}
	return d
}
