package spec

import (
	"cmp"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"go.senan.xyz/gonic/db"
)

func NewAlbumByTags(a *db.Album, artists []*db.Artist) *Album {
	ret := &Album{
		ID:            a.SID(),
		Created:       a.CreatedAt,
		Artists:       []*ArtistRef{},
		DisplayArtist: a.TagAlbumArtist,
		Title:         a.TagTitle,
		Album:         a.TagTitle,
		Name:          a.TagTitle,
		TrackCount:    a.ChildCount,
		Duration:      a.Duration,
		Genres:        []*GenreRef{},
		Year:          a.TagYear,
		Tracks:        []*TrackChild{},
		AverageRating: a.AverageRating,
		IsCompilation: a.TagCompilation,
		ReleaseTypes:  formatReleaseTypes(a.TagReleaseType),
		DiscTitles:    []*DiscTitle{},
	}
	if a.Cover != "" {
		ret.CoverID = a.SID()
	} else if a.EmbeddedCoverTrackID != nil {
		ret.CoverID = a.EmbeddedCoverTrackSID()
	}
	if a.AlbumStar != nil {
		ret.Starred = &a.AlbumStar.StarDate
	}
	if a.AlbumRating != nil {
		ret.UserRating = a.AlbumRating.Rating
	}
	sort.Slice(artists, func(i, j int) bool {
		return artists[i].ID < artists[j].ID
	})
	if len(artists) > 0 {
		ret.Artist = artists[0].Name
		ret.ArtistID = artists[0].SID()
	}
	for _, a := range artists {
		ret.Artists = append(ret.Artists, &ArtistRef{
			ID:   a.SID(),
			Name: a.Name,
		})
	}
	if len(a.Genres) > 0 {
		ret.Genre = a.Genres[0].Name
	}
	for _, g := range a.Genres {
		ret.Genres = append(ret.Genres, &GenreRef{Name: g.Name})
	}
	if a.Play != nil {
		ret.PlayCount = a.Play.Count
	}
	if len(a.DiscTitles) > 0 {
		sort.Slice(a.DiscTitles, func(i, j int) bool {
			return a.DiscTitles[i].DiscNumber < a.DiscTitles[j].DiscNumber
		})
		for _, dt := range a.DiscTitles {
			ret.DiscTitles = append(ret.DiscTitles, &DiscTitle{
				Disc:  dt.DiscNumber,
				Title: dt.Title,
			})
		}
	}
	return ret
}

func NewTrackByTags(t *db.Track, album *db.Album) *TrackChild {
	ret := &TrackChild{
		ID:                 t.SID(),
		Album:              album.TagTitle,
		AlbumID:            album.SID(),
		Artists:            []*ArtistRef{},
		DisplayArtist:      t.TagTrackArtist,
		AlbumArtists:       []*ArtistRef{},
		AlbumDisplayArtist: album.TagAlbumArtist,
		Bitrate:            t.Bitrate,
		ContentType:        t.MIME(),
		CreatedAt:          t.CreatedAt,
		Duration:           t.Length,
		Genres:             []*GenreRef{},
		ParentID:           t.AlbumSID(),
		Path:               filepath.Join(album.LeftPath, album.RightPath, t.Filename),
		Size:               t.Size,
		Suffix:             formatExt(t.Ext()),
		Title:              cmp.Or(t.TagTitle, t.Filename),
		TrackNumber:        t.TagTrackNumber,
		DiscNumber:         t.TagDiscNumber,
		Type:               "music",
		MusicBrainzID:      t.TagBrainzID,
		AverageRating:      t.AverageRating,
		TranscodeMeta:      TranscodeMeta{},
		Year:               t.TagYear,
	}

	switch {
	case t.HasEmbeddedCover:
		ret.CoverID = t.SID()
	case album.Cover != "":
		ret.CoverID = album.SID()
	case album.EmbeddedCoverTrackID != nil:
		ret.CoverID = album.EmbeddedCoverTrackSID()
	}

	if t.TrackStar != nil {
		ret.Starred = &t.TrackStar.StarDate
	}
	if t.TrackRating != nil {
		ret.UserRating = t.TrackRating.Rating
	}

	slices.SortFunc(t.Artists, func(a, b *db.Artist) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(album.Artists, func(a, b *db.Artist) int { return cmp.Compare(a.ID, b.ID) })

	switch {
	case len(t.Artists) > 0:
		ret.Artist = t.Artists[0].Name
		ret.ArtistID = t.Artists[0].SID()
	case len(album.Artists) > 0:
		ret.Artist = album.Artists[0].Name
		ret.ArtistID = album.Artists[0].SID()
	}
	for _, a := range t.Artists {
		ret.Artists = append(ret.Artists, &ArtistRef{ID: a.SID(), Name: a.Name})
	}
	if len(t.Genres) > 0 {
		ret.Genre = t.Genres[0].Name
	}
	for _, g := range t.Genres {
		ret.Genres = append(ret.Genres, &GenreRef{Name: g.Name})
	}
	for _, a := range album.Artists {
		ret.AlbumArtists = append(ret.AlbumArtists, &ArtistRef{ID: a.SID(), Name: a.Name})
	}

	slices.SortStableFunc(t.Contributors, func(a, b *db.TrackContributor) int {
		return cmp.Or(
			cmp.Compare(a.Role, b.Role),
			cmp.Compare(a.ArtistID, b.ArtistID),
		)
	})

	var composers []string
	for _, c := range t.Contributors {
		if c.Artist == nil {
			continue
		}
		ret.Contributors = append(ret.Contributors, &Contributor{
			Role:   string(c.Role),
			Artist: &ArtistRef{ID: c.Artist.SID(), Name: c.Artist.Name},
		})
		if c.Role == db.ContributorRoleComposer {
			composers = append(composers, c.Artist.Name)
		}
	}
	ret.DisplayComposer = strings.Join(composers, ", ")

	if t.ReplayGainTrackGain != 0 || t.ReplayGainAlbumGain != 0 {
		ret.ReplayGain = &ReplayGain{
			TrackGain: t.ReplayGainTrackGain,
			TrackPeak: t.ReplayGainTrackPeak,
			AlbumGain: t.ReplayGainAlbumGain,
			AlbumPeak: t.ReplayGainAlbumPeak,
		}
	}
	return ret
}

func NewArtistByTags(a *db.Artist) *Artist {
	r := &Artist{
		ID:            a.SID(),
		Name:          a.Name,
		AlbumCount:    a.AlbumCount,
		Albums:        []*Album{},
		AverageRating: a.AverageRating,
	}
	if a.Info != nil && a.Info.ImageURL != "" {
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
