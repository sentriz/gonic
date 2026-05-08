package spec

import (
	"cmp"
	"path/filepath"
	"sort"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
)

func LoadTrackByFolder(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Scopes(TrackWithArtistCredits, TrackWithUserData(userID)).
			Preload("Album").
			Preload("Genres").
			Preload("ISRCs")
	}
}

func NewAlbumByFolder(f *AlbumRow) *Album {
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
		AverageRating: f.AverageRating,
		ReleaseTypes:  []string{},
	}
	if f.AlbumStar != nil {
		a.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		a.UserRating = f.AlbumRating.Rating
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	} else if f.EmbeddedCoverTrackID != nil {
		a.CoverID = f.EmbeddedCoverTrackSID()
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
		AverageRating: f.AverageRating,
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
	} else if f.EmbeddedCoverTrackID != nil {
		trCh.CoverID = f.EmbeddedCoverTrackSID()
	}

	return trCh
}

func NewTCTrackByFolder(t *db.Track, parent *db.Album) *TrackChild {
	trCh := &TrackChild{
		ID:            t.SID(),
		ContentType:   t.MIME(),
		Suffix:        formatExt(t.Ext()),
		Size:          t.Size,
		DisplayArtist: t.TagTrackArtist,
		Title:         cmp.Or(t.TagTitle, t.Filename),
		TrackNumber:   t.TagTrackNumber,
		DiscNumber:    t.TagDiscNumber,
		Path: filepath.Join(
			parent.LeftPath,
			parent.RightPath,
			t.Filename,
		),
		ParentID:      parent.SID(),
		Duration:      t.Length,
		Bitrate:       t.Bitrate,
		IsDir:         false,
		Type:          "music",
		MusicBrainzID: t.TagBrainzID,
		CreatedAt:     t.CreatedAt,
		AverageRating: t.AverageRating,
		Year:          t.TagYear,
	}
	if trCh.Title == "" {
		trCh.Title = t.Filename
	}

	switch {
	case t.HasEmbeddedCover:
		trCh.CoverID = t.SID()
	case parent.Cover != "":
		trCh.CoverID = parent.SID()
	case parent.EmbeddedCoverTrackID != nil:
		trCh.CoverID = parent.EmbeddedCoverTrackSID()
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
	for _, trI := range t.ISRCs {
		trCh.ISRC = append(trCh.ISRC, trI.ISRC)
	}
	trackArtists := filterTrackCreditsByRole(t.Credits, db.RoleArtist)
	sort.Slice(trackArtists, func(i, j int) bool {
		return trackArtists[i].ArtistID < trackArtists[j].ArtistID
	})
	if len(trackArtists) > 0 && trackArtists[0].Artist != nil {
		trCh.Artist = cmp.Or(trackArtists[0].CreditedAs, trackArtists[0].Artist.Name)
		trCh.ArtistID = trackArtists[0].Artist.SID()
	}
	for _, c := range trackArtists {
		if c.Artist == nil {
			continue
		}
		trCh.Artists = append(trCh.Artists, &ArtistRef{ID: c.Artist.SID(), Name: cmp.Or(c.CreditedAs, c.Artist.Name)})
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
		Album:       pe.Album,
		Artist:      pe.Artist,
		CoverID:     pe.SID(),
	}
	if pe.Podcast != nil {
		trCh.ParentID = pe.Podcast.SID()
		trCh.Path = pe.AbsPath()
	}
	return trCh
}

func NewArtistByFolder(f *AlbumRow) *Artist {
	// the db is structured around "browse by tags", and where
	// an album is also a folder. so we're constructing an artist
	// from an "album" where
	// maybe TODO: rename the Album model to Folder
	a := &Artist{
		ID:            f.SID(),
		Name:          f.RightPath,
		AlbumCount:    f.ChildCount,
		Roles:         []string{},
		AverageRating: f.AverageRating,
	}
	if f.AlbumStar != nil {
		a.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		a.UserRating = f.AlbumRating.Rating
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	} else if f.EmbeddedCoverTrackID != nil {
		a.CoverID = f.EmbeddedCoverTrackSID()
	}
	return a
}

func NewDirectoryByFolder(f *db.Album, children []*TrackChild) *Directory {
	d := &Directory{
		ID:            f.SID(),
		Name:          f.RightPath,
		Children:      children,
		ParentID:      f.ParentSID(),
		AverageRating: f.AverageRating,
	}
	if f.AlbumStar != nil {
		d.Starred = &f.AlbumStar.StarDate
	}
	if f.AlbumRating != nil {
		d.UserRating = f.AlbumRating.Rating
	}
	return d
}
