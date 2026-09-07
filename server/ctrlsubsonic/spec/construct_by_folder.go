//nolint:goconst
package spec

import (
	"cmp"
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"go.senan.xyz/gonic/db"
)

// TrackChildrenByFolder renders tracks for the browse-by-folder endpoints,
// which name only the performing artists.
func TrackChildrenByFolder(r Render, tracks []*db.Track) ([]*TrackChild, error) {
	extras, err := loadTrackExtras(r, tracks, []string{db.RoleArtist})
	if err != nil {
		return nil, err
	}
	ret := make([]*TrackChild, 0, len(tracks))
	for _, track := range tracks {
		child := newTCTrackByFolder(track, extras[track.ID])
		child.TranscodeMeta = r.TranscodeMeta
		ret = append(ret, child)
	}
	return ret, nil
}

// AlbumsByFolder renders folder rows that stand for real albums, so unlike the
// other browse-by-folder renderers these show a song count and duration.
func AlbumsByFolder(r Render, albums []*db.Album) ([]*Album, error) {
	keys := ids(albums, func(a *db.Album) int { return a.ID })

	user, err := loadAlbumUserExtras(r, keys)
	if err != nil {
		return nil, err
	}
	trackStats, err := loadAlbumTrackStats(r.DB, keys)
	if err != nil {
		return nil, fmt.Errorf("load album track stats: %w", err)
	}
	plays, err := loadAlbumPlays(r.DB, r.UserID, keys)
	if err != nil {
		return nil, fmt.Errorf("load album plays: %w", err)
	}
	parents, err := db.FindByID(r.DB.DB, "id", ids(albums, func(a *db.Album) int { return a.ParentID }),
		func(a *db.Album) int { return a.ID })
	if err != nil {
		return nil, fmt.Errorf("load album parents: %w", err)
	}

	ret := make([]*Album, 0, len(albums))
	for _, album := range albums {
		x := user[album.ID]
		x.parent = parents[album.ParentID]
		x.withTrackStats(trackStats[album.ID], plays[album.ID])
		ret = append(ret, newAlbumByFolder(album, x))
	}
	return ret, nil
}

// TCAlbumsByFolder renders folder rows listed as the children of a directory.
func TCAlbumsByFolder(r Render, albums []*db.Album) ([]*TrackChild, error) {
	user, err := loadAlbumUserExtras(r, ids(albums, func(a *db.Album) int { return a.ID }))
	if err != nil {
		return nil, err
	}
	ret := make([]*TrackChild, 0, len(albums))
	for _, album := range albums {
		ret = append(ret, newTCAlbumByFolder(album, user[album.ID]))
	}
	return ret, nil
}

// ArtistsByFolder renders the top level folders, which browse-by-folder shows
// as artists. They count their child albums rather than their own tracks.
func ArtistsByFolder(r Render, albums []*db.Album) ([]*Artist, error) {
	keys := ids(albums, func(a *db.Album) int { return a.ID })

	user, err := loadAlbumUserExtras(r, keys)
	if err != nil {
		return nil, err
	}
	childCounts, err := loadAlbumChildCounts(r.DB, keys)
	if err != nil {
		return nil, fmt.Errorf("load album child counts: %w", err)
	}

	ret := make([]*Artist, 0, len(albums))
	for _, album := range albums {
		x := user[album.ID]
		x.childCount = childCounts[album.ID]
		ret = append(ret, newArtistByFolder(album, x))
	}
	return ret, nil
}

// DirectoriesByFolder renders folder rows as directories. Callers that have
// children for them fill Children in afterwards.
func DirectoriesByFolder(r Render, albums []*db.Album) ([]*Directory, error) {
	user, err := loadAlbumUserExtras(r, ids(albums, func(a *db.Album) int { return a.ID }))
	if err != nil {
		return nil, err
	}
	ret := make([]*Directory, 0, len(albums))
	for _, album := range albums {
		ret = append(ret, newDirectoryByFolder(album, user[album.ID]))
	}
	return ret, nil
}

func newAlbumByFolder(f *db.Album, x albumExtras) *Album {
	a := &Album{
		Artist:        x.parent.RightPath,
		ID:            f.SID(),
		IsDir:         true,
		ParentID:      f.ParentSID(),
		Album:         f.RightPath,
		Name:          f.RightPath,
		Title:         f.RightPath,
		TrackCount:    x.trackCount,
		Duration:      x.duration,
		Created:       f.CreatedAt,
		AverageRating: x.averageRating,
		PlayCount:     int(math.Ceil(x.playCount)),
		Played:        Time{x.playTime.Time},
		Artists:       []*ArtistRef{},
		ReleaseTypes:  []string{},
		RecordLabels:  []*RecordLabel{},
		DiscTitles:    []*DiscTitle{},
		Genres:        []*GenreRef{},
	}
	if x.star != nil {
		a.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		a.UserRating = x.rating.Rating
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	} else if f.EmbeddedCoverTrackID != nil {
		a.CoverID = f.EmbeddedCoverTrackSID()
	}
	return a
}

func newTCAlbumByFolder(f *db.Album, x albumExtras) *TrackChild {
	trCh := &TrackChild{
		ID:            f.SID(),
		IsDir:         true,
		MediaType:     MediaTypeAlbum,
		Title:         f.RightPath,
		ParentID:      f.ParentSID(),
		CreatedAt:     f.CreatedAt,
		AverageRating: x.averageRating,
		Year:          f.TagYear,
		Artists:       []*ArtistRef{},
		AlbumArtists:  []*ArtistRef{},
		Contributors:  []*Contributor{},
		Genres:        []*GenreRef{},
		ISRC:          []string{},
	}
	if x.star != nil {
		trCh.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		trCh.UserRating = x.rating.Rating
	}
	if f.Cover != "" {
		trCh.CoverID = f.SID()
	} else if f.EmbeddedCoverTrackID != nil {
		trCh.CoverID = f.EmbeddedCoverTrackSID()
	}

	return trCh
}

func newTCTrackByFolder(t *db.Track, x trackExtras) *TrackChild {
	parent := x.album
	trCh := &TrackChild{
		ID:              t.SID(),
		ContentType:     t.MIME(),
		Suffix:          formatExt(t.Ext()),
		Size:            t.Size,
		Artists:         []*ArtistRef{},
		AlbumArtists:    []*ArtistRef{},
		Contributors:    []*Contributor{},
		Genres:          []*GenreRef{},
		ISRC:            []string{},
		DisplayArtist:   cmp.Or(t.TagTrackArtistCredit, t.TagTrackArtist),
		DisplayComposer: cmp.Or(t.TagComposerCredit, t.TagComposer),
		Title:           cmp.Or(t.TagTitle, t.Filename),
		TrackNumber:     t.TagTrackNumber,
		DiscNumber:      t.TagDiscNumber,
		Path: filepath.Join(
			parent.LeftPath,
			parent.RightPath,
			t.Filename,
		),
		ParentID:      parent.SID(),
		Duration:      t.Length,
		Bitrate:       t.Bitrate,
		IsDir:         false,
		Type:          TypeMusic,
		MediaType:     MediaTypeSong,
		MusicBrainzID: t.TagBrainzID,
		CreatedAt:     t.CreatedAt,
		AverageRating: x.averageRating,
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

	if x.album != nil {
		trCh.Album = x.album.RightPath
		trCh.AlbumID = x.album.SID()
	}
	if x.star != nil {
		trCh.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		trCh.UserRating = x.rating.Rating
	}
	if x.play != nil {
		trCh.PlayCount = int(math.Ceil(x.play.Count))
		trCh.Played = Time{x.play.Time}
	}
	if len(x.genres) > 0 {
		trCh.Genre = x.genres[0].Name
	}
	for _, g := range x.genres {
		trCh.Genres = append(trCh.Genres, &GenreRef{Name: g.Name})
	}
	for _, trI := range x.isrcs {
		trCh.ISRC = append(trCh.ISRC, trI.ISRC)
	}
	trackArtists := filterTrackCreditsByRole(x.credits, db.RoleArtist)
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
		ID:           pe.SID(),
		ContentType:  pe.MIME(),
		Suffix:       pe.Ext(),
		Size:         pe.Size,
		Title:        pe.Title,
		ParentID:     pe.SID(),
		Duration:     pe.Length,
		Bitrate:      pe.Bitrate,
		IsDir:        false,
		Type:         TypePodcastEpisode,
		MediaType:    MediaTypeSong,
		CreatedAt:    pe.CreatedAt,
		Album:        pe.Album,
		Artist:       pe.Artist,
		CoverID:      pe.SID(),
		Artists:      []*ArtistRef{},
		AlbumArtists: []*ArtistRef{},
		Contributors: []*Contributor{},
		ISRC:         []string{},
		Genres:       []*GenreRef{},
	}
	if pe.Podcast != nil {
		trCh.ParentID = pe.Podcast.SID()
		trCh.Path = pe.AbsPath()
	}
	return trCh
}

func newArtistByFolder(f *db.Album, x albumExtras) *Artist {
	// the db is structured around "browse by tags", and where
	// an album is also a folder. so we're constructing an artist
	// from an "album" where
	// maybe TODO: rename the Album model to Folder
	a := &Artist{
		ID:            f.SID(),
		Name:          f.RightPath,
		AlbumCount:    x.childCount,
		Roles:         []string{},
		AverageRating: x.averageRating,
	}
	if x.star != nil {
		a.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		a.UserRating = x.rating.Rating
	}
	if f.Cover != "" {
		a.CoverID = f.SID()
	} else if f.EmbeddedCoverTrackID != nil {
		a.CoverID = f.EmbeddedCoverTrackSID()
	}
	return a
}

func newDirectoryByFolder(f *db.Album, x albumExtras) *Directory {
	d := &Directory{
		ID:            f.SID(),
		Name:          f.RightPath,
		ParentID:      f.ParentSID(),
		AverageRating: x.averageRating,
	}
	if x.star != nil {
		d.Starred = &x.star.StarDate
	}
	if x.rating != nil {
		d.UserRating = x.rating.Rating
	}
	return d
}
