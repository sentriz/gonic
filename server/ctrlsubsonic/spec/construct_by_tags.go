package spec

import (
	"cmp"
	"math"
	"path/filepath"
	"slices"
	"sort"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
)

func LoadAlbumByTags(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Scopes(AlbumWithUserPlay(userID), AlbumWithAlbumArtistCredits, AlbumWithUserData(userID)).
			Preload("Genres").
			Preload("Labels").
			Preload("DiscTitles")
	}
}

func LoadTrackByTags(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Scopes(TrackWithAlbumArtistCredits, TrackWithUserData(userID)).
			Preload("Album").
			Preload("Credits.Artist").
			Preload("Genres").
			Preload("ISRCs").
			Preload("Play", "user_id=?", userID)
	}
}

func LoadArtistByTags(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Scopes(ArtistWithRolesAndAlbumCount, ArtistWithUserData(userID)).
			Preload("Info")
	}
}

func NewAlbumByTags(a *AlbumRow, credits []*db.AlbumCredit) *Album {
	ret := &Album{
		ID:            a.SID(),
		Created:       a.CreatedAt,
		Artists:       []*ArtistRef{},
		DisplayArtist: cmp.Or(a.TagAlbumArtistCredit, a.TagAlbumArtist),
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
		RecordLabels:  []*RecordLabel{},
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

	albumArtists := filterAlbumCreditsByRole(credits, db.RoleAlbumArtist)
	slices.SortFunc(albumArtists, func(a, b *db.AlbumCredit) int { return cmp.Compare(a.ArtistID, b.ArtistID) })

	if len(albumArtists) > 0 && albumArtists[0].Artist != nil {
		ret.Artist = cmp.Or(albumArtists[0].CreditedAs, albumArtists[0].Artist.Name)
		ret.ArtistID = albumArtists[0].Artist.SID()
	}
	for _, c := range albumArtists {
		if c.Artist == nil {
			continue
		}
		ret.Artists = append(ret.Artists, &ArtistRef{
			ID:   c.Artist.SID(),
			Name: cmp.Or(c.CreditedAs, c.Artist.Name),
		})
	}
	if len(a.Genres) > 0 {
		ret.Genre = a.Genres[0].Name
	}
	for _, g := range a.Genres {
		ret.Genres = append(ret.Genres, &GenreRef{Name: g.Name})
	}
	for _, l := range a.Labels {
		ret.RecordLabels = append(ret.RecordLabels, &RecordLabel{Name: l.Label})
	}
	ret.PlayCount = int(math.Ceil(a.PlayCount))
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

func NewTrackByTags(client string, t *db.Track, album *db.Album) *TrackChild {
	ret := &TrackChild{
		ID:                 t.SID(),
		Album:              album.TagTitle,
		AlbumID:            album.SID(),
		Artists:            []*ArtistRef{},
		DisplayArtist:      cmp.Or(t.TagTrackArtistCredit, t.TagTrackArtist),
		AlbumArtists:       []*ArtistRef{},
		AlbumDisplayArtist: cmp.Or(album.TagAlbumArtistCredit, album.TagAlbumArtist),
		Bitrate:            t.Bitrate,
		ContentType:        t.MIME(),
		CreatedAt:          t.CreatedAt,
		Duration:           t.Length,
		Genres:             []*GenreRef{},
		ISRC:               []string{},
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
	if t.Play != nil {
		ret.PlayCount = int(math.Ceil(t.Play.Count))
	}

	trackArtists := filterTrackCreditsByRole(t.Credits, db.RoleArtist)
	slices.SortFunc(trackArtists, func(a, b *db.TrackCredit) int { return cmp.Compare(a.ArtistID, b.ArtistID) })

	albumArtists := filterAlbumCreditsByRole(album.Credits, db.RoleAlbumArtist)
	slices.SortFunc(albumArtists, func(a, b *db.AlbumCredit) int { return cmp.Compare(a.ArtistID, b.ArtistID) })

	switch {
	case len(trackArtists) > 0 && trackArtists[0].Artist != nil:
		ret.Artist = cmp.Or(trackArtists[0].CreditedAs, trackArtists[0].Artist.Name)
		ret.ArtistID = trackArtists[0].Artist.SID()
	case len(albumArtists) > 0 && albumArtists[0].Artist != nil:
		ret.Artist = cmp.Or(albumArtists[0].CreditedAs, albumArtists[0].Artist.Name)
		ret.ArtistID = albumArtists[0].Artist.SID()
	}
	for _, c := range trackArtists {
		if c.Artist == nil {
			continue
		}
		ret.Artists = append(ret.Artists, &ArtistRef{ID: c.Artist.SID(), Name: cmp.Or(c.CreditedAs, c.Artist.Name)})
	}
	if len(t.Genres) > 0 {
		ret.Genre = t.Genres[0].Name
	}
	for _, g := range t.Genres {
		ret.Genres = append(ret.Genres, &GenreRef{Name: g.Name})
	}
	for _, trI := range t.ISRCs {
		ret.ISRC = append(ret.ISRC, trI.ISRC)
	}
	for _, c := range albumArtists {
		if c.Artist == nil {
			continue
		}
		ret.AlbumArtists = append(ret.AlbumArtists, &ArtistRef{ID: c.Artist.SID(), Name: cmp.Or(c.CreditedAs, c.Artist.Name)})
	}

	// DSub treats nested <artist> elements as top-level artists, so the <artist> inside
	// <contributors> shows up as a phantom artist in search results and overwrites the
	// directory header in album views.
	if client != "DSub" {
		var contributors []*db.TrackCredit
		for _, c := range t.Credits {
			switch c.Role {
			case db.RoleArtist, db.RoleAlbumArtist:
			default:
				contributors = append(contributors, c)
			}
		}
		slices.SortStableFunc(contributors, func(a, b *db.TrackCredit) int {
			return cmp.Or(
				cmp.Compare(a.Role, b.Role),
				cmp.Compare(a.ArtistID, b.ArtistID),
			)
		})

		for _, c := range contributors {
			if c.Artist == nil {
				continue
			}
			ret.Contributors = append(ret.Contributors, &Contributor{
				Role:   c.Role,
				Artist: &ArtistRef{ID: c.Artist.SID(), Name: cmp.Or(c.CreditedAs, c.Artist.Name)},
			})
		}
	}

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

func NewArtistByTags(a *ArtistRow) *Artist {
	roles := a.GetRoles()
	if roles == nil {
		roles = []string{}
	}
	r := &Artist{
		ID:            a.SID(),
		Name:          a.Name,
		AlbumCount:    a.AlbumCount,
		Roles:         roles,
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

func filterAlbumCreditsByRole(credits []*db.AlbumCredit, role string) []*db.AlbumCredit {
	out := make([]*db.AlbumCredit, 0, len(credits))
	for _, c := range credits {
		if c.Role == role {
			out = append(out, c)
		}
	}
	return out
}

func filterTrackCreditsByRole(credits []*db.TrackCredit, role string) []*db.TrackCredit {
	out := make([]*db.TrackCredit, 0, len(credits))
	for _, c := range credits {
		if c.Role == role {
			out = append(out, c)
		}
	}
	return out
}

func NewGenre(g *GenreRow) *Genre {
	return &Genre{
		Name:       g.Name,
		AlbumCount: g.AlbumCount,
		SongCount:  g.TrackCount,
	}
}
