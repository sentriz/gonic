package spec

import (
	"cmp"
	"fmt"
	"math"
	"path/filepath"
	"slices"
	"sort"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

// AlbumsByTags renders albums for the browse-by-tags endpoints. It loads what
// the rendering reads, so callers pass rows straight from a query.
func AlbumsByTags(r Render, albums []*db.Album) ([]*Album, error) {
	extras, err := loadAlbumExtras(r, albums)
	if err != nil {
		return nil, err
	}
	ret := make([]*Album, 0, len(albums))
	for _, album := range albums {
		ret = append(ret, newAlbumByTags(album, extras[album.ID]))
	}
	return ret, nil
}

// ArtistsByTags renders artists for the browse-by-tags endpoints. The music
// folder is the one the artists were found under, and narrows their album
// counts to match.
func ArtistsByTags(r Render, artists []*db.Artist) ([]*Artist, error) {
	extras, err := loadArtistExtras(r, artists)
	if err != nil {
		return nil, err
	}
	ret := make([]*Artist, 0, len(artists))
	for _, artist := range artists {
		ret = append(ret, newArtistByTags(artist, extras[artist.ID]))
	}
	return ret, nil
}

// TrackChildrenByTags renders tracks for the browse-by-tags endpoints.
func TrackChildrenByTags(r Render, tracks []*db.Track) ([]*TrackChild, error) {
	extras, err := loadTrackExtras(r, tracks, nil)
	if err != nil {
		return nil, err
	}
	albumCredits, err := loadAlbumCredits(r.DB, ids(tracks, func(t *db.Track) int { return t.AlbumID }), db.RoleAlbumArtist)
	if err != nil {
		return nil, fmt.Errorf("load album credits: %w", err)
	}
	ret := make([]*TrackChild, 0, len(tracks))
	for _, track := range tracks {
		x := extras[track.ID]
		x.albumCredits = albumCredits[track.AlbumID]
		child := newTrackByTags(r.Client, track, x)
		child.TranscodeMeta = r.TranscodeMeta
		ret = append(ret, child)
	}
	return ret, nil
}

func newAlbumByTags(a *db.Album, x albumExtras) *Album {
	credits := x.credits
	ret := &Album{
		ID:            a.SID(),
		Created:       a.CreatedAt,
		Artists:       []*ArtistRef{},
		DisplayArtist: cmp.Or(a.TagAlbumArtistCredit, a.TagAlbumArtist),
		Title:         a.TagTitle,
		Album:         a.TagTitle,
		Name:          a.TagTitle,
		TrackCount:    x.trackCount,
		Duration:      x.duration,
		Genres:        []*GenreRef{},
		Year:          a.TagYear,
		Tracks:        []*TrackChild{},
		AverageRating: x.averageRating,
		IsCompilation: a.TagCompilation,
		ReleaseTypes:  formatReleaseTypes(a.TagReleaseType),
		MusicBrainzID: a.TagBrainzID,
		Version:       a.TagVersion,
		RecordLabels:  []*RecordLabel{},
		DiscTitles:    []*DiscTitle{},
	}
	if a.Cover != "" {
		ret.CoverID = a.SID()
	} else if a.EmbeddedCoverTrackID != nil {
		ret.CoverID = a.EmbeddedCoverTrackSID()
	}
	if x.star != nil {
		ret.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		ret.UserRating = x.rating.Rating
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
	if len(x.genres) > 0 {
		ret.Genre = x.genres[0].Name
	}
	for _, g := range x.genres {
		ret.Genres = append(ret.Genres, &GenreRef{Name: g.Name})
	}
	for _, l := range x.labels {
		ret.RecordLabels = append(ret.RecordLabels, &RecordLabel{Name: l.Label})
	}
	ret.PlayCount = int(math.Ceil(x.playCount))
	ret.Played = Time{x.playTime.Time}
	if len(x.discTitles) > 0 {
		sort.Slice(x.discTitles, func(i, j int) bool {
			return x.discTitles[i].DiscNumber < x.discTitles[j].DiscNumber
		})
		for _, dt := range x.discTitles {
			ret.DiscTitles = append(ret.DiscTitles, &DiscTitle{
				Disc:  dt.DiscNumber,
				Title: dt.Title,
			})
		}
	}
	return ret
}

func newTrackByTags(client string, t *db.Track, x trackExtras) *TrackChild {
	album := x.album
	ret := &TrackChild{
		ID:                 t.SID(),
		Album:              album.TagTitle,
		AlbumID:            album.SID(),
		Artists:            []*ArtistRef{},
		DisplayArtist:      cmp.Or(t.TagTrackArtistCredit, t.TagTrackArtist),
		AlbumArtists:       []*ArtistRef{},
		AlbumDisplayArtist: cmp.Or(album.TagAlbumArtistCredit, album.TagAlbumArtist),
		Contributors:       []*Contributor{},
		DisplayComposer:    cmp.Or(t.TagComposerCredit, t.TagComposer),
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
		Type:               TypeMusic,
		MediaType:          MediaTypeSong,
		MusicBrainzID:      t.TagBrainzID,
		AverageRating:      x.averageRating,
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

	if x.star != nil {
		ret.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		ret.UserRating = x.rating.Rating
	}
	if x.play != nil {
		ret.PlayCount = int(math.Ceil(x.play.Count))
		ret.Played = Time{x.play.Time}
	}

	trackArtists := filterTrackCreditsByRole(x.credits, db.RoleArtist)
	slices.SortFunc(trackArtists, func(a, b *db.TrackCredit) int { return cmp.Compare(a.ArtistID, b.ArtistID) })

	albumArtists := filterAlbumCreditsByRole(x.albumCredits, db.RoleAlbumArtist)
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
	if len(x.genres) > 0 {
		ret.Genre = x.genres[0].Name
	}
	for _, g := range x.genres {
		ret.Genres = append(ret.Genres, &GenreRef{Name: g.Name})
	}
	for _, trI := range x.isrcs {
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
		for _, c := range x.credits {
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

func newArtistByTags(a *db.Artist, x artistExtras) *Artist {
	roles := x.roles
	if roles == nil {
		roles = []string{}
	}
	r := &Artist{
		ID:            a.SID(),
		Name:          a.Name,
		AlbumCount:    x.albumCount,
		MusicBrainzID: a.MusicBrainzID,
		Roles:         roles,
		Albums:        []*Album{},
		AverageRating: x.averageRating,
	}
	if x.info != nil {
		r.Disambiguation = x.info.MusicBrainzDisambiguation
		if x.info.ImageURL != "" {
			r.CoverID = a.SID()
		}
	}
	if r.CoverID == nil && x.coverAlbumID != 0 {
		r.CoverID = &specid.ID{Type: specid.Album, Value: x.coverAlbumID}
	}
	if x.star != nil {
		r.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		r.UserRating = x.rating.Rating
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

// GenresWithCounts renders genres along with how much each one covers.
func GenresWithCounts(dbc *db.DB, genres []*db.Genre) ([]*Genre, error) {
	keys := ids(genres, func(g *db.Genre) int { return g.ID })

	albumCounts, err := loadGenreCounts(dbc, "album_genres", keys)
	if err != nil {
		return nil, fmt.Errorf("load genre album counts: %w", err)
	}
	trackCounts, err := loadGenreCounts(dbc, "track_genres", keys)
	if err != nil {
		return nil, fmt.Errorf("load genre track counts: %w", err)
	}

	ret := make([]*Genre, 0, len(genres))
	for _, genre := range genres {
		ret = append(ret, &Genre{
			Name:       genre.Name,
			AlbumCount: albumCounts[genre.ID],
			SongCount:  trackCounts[genre.ID],
		})
	}
	return ret, nil
}
