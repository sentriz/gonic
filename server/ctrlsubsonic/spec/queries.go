package spec

import (
	"fmt"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
)

// This file loads what the renderers read beyond a row's own columns: what the
// user has done with it, what it adds up to, and what it is related to. Each
// entity has one loader gathering all of it, so a renderer asks once and the
// cost of rendering that entity is visible in one place.
//
// Everything is loaded by a set of ids rather than by joining onto the query
// that found the rows, both because a join would run during the scan - before
// any LIMIT - and because gorm emits one bind variable per parent row, which
// SQLite refuses past 32766 of them. db.FindByID chunks around that.

// Render is what every renderer needs to know about the request it is
// answering: which database, which user is asking, and how that user's client
// wants its media described. Handlers build one per request and pass it to
// whatever they render, so nothing has to be threaded a piece at a time.
type Render struct {
	DB            *db.DB
	UserID        int
	Client        string
	TranscodeMeta TranscodeMeta
	MusicFolder   string
}

// Extras
//
// What a renderer reads about a row beyond the row's own columns. These are
// built and read entirely within this package, so unlike a row struct embedding
// a db model they never meet gorm: no tags, and no column of a joined table can
// land in one by accident.

type artistExtras struct {
	star          *db.ArtistStar
	rating        *db.ArtistRating
	info          *db.ArtistInfo
	roles         []string
	averageRating float64
	albumCount    int
	coverAlbumID  int
}

type albumExtras struct {
	star          *db.AlbumStar
	rating        *db.AlbumRating
	credits       []*db.AlbumCredit
	genres        []*db.Genre
	labels        []*db.AlbumLabel
	discTitles    []*db.AlbumDiscTitle
	parent        *db.Album
	averageRating float64
	trackCount    int
	duration      int
	playCount     float64
	playTime      db.ScanTime
	childCount    int
}

type trackExtras struct {
	album         *db.Album
	star          *db.TrackStar
	rating        *db.TrackRating
	play          *db.TrackPlay
	credits       []*db.TrackCredit
	albumCredits  []*db.AlbumCredit
	genres        []*db.Genre
	isrcs         []*db.TrackISRC
	averageRating float64
}

// loadArtistExtras gathers what the artist renderer reads. The music folder
// narrows the album counts the same way it narrows the query that found the
// artists, so an artist listed under one folder counts only its albums there.
func loadArtistExtras(r Render, artists []*db.Artist) (map[int]artistExtras, error) {
	keys := ids(artists, func(a *db.Artist) int { return a.ID })
	mine := r.DB.Where("user_id=?", r.UserID)

	stars, err := db.FindByID(mine, "artist_id", keys, func(s *db.ArtistStar) int { return s.ArtistID })
	if err != nil {
		return nil, fmt.Errorf("load stars: %w", err)
	}
	ratings, err := db.FindByID(mine, "artist_id", keys, func(r *db.ArtistRating) int { return r.ArtistID })
	if err != nil {
		return nil, fmt.Errorf("load ratings: %w", err)
	}
	infos, err := db.FindByID(r.DB.DB, "id", keys, func(i *db.ArtistInfo) int { return i.ID })
	if err != nil {
		return nil, fmt.Errorf("load infos: %w", err)
	}
	averageRatings, err := loadAverageRatings(r.DB, db.ArtistRating{}, "artist_id", keys)
	if err != nil {
		return nil, fmt.Errorf("load average ratings: %w", err)
	}
	roles, err := loadArtistRoles(r.DB, keys)
	if err != nil {
		return nil, fmt.Errorf("load roles: %w", err)
	}
	albumCounts, err := scanCounts(r.DB.
		Model(db.AlbumCredit{}).
		Select("artist_id AS id, count(album_credits.album_id) count").
		Joins("JOIN albums ON albums.id=album_credits.album_id").
		Where("role=?", db.RoleAlbumArtist).
		Scopes(WithAlbumRootDir(r.MusicFolder)).
		Group("artist_id"),
		"artist_id", keys)
	if err != nil {
		return nil, fmt.Errorf("load album counts: %w", err)
	}

	type artistCover struct {
		ID           int
		CoverAlbumID int
	}
	covers, err := scanByID(r.DB.Model(db.Artist{}).Select(`artists.id AS id, coalesce(
		(SELECT albums.id FROM albums
			JOIN album_credits ON album_credits.album_id=albums.id
			WHERE album_credits.artist_id=artists.id AND album_credits.role='albumartist' AND albums.cover<>''
			ORDER BY albums.tag_year DESC, albums.id
			LIMIT 1),
		(SELECT albums.id FROM albums WHERE albums.cover<>'' AND albums.id IN (
			SELECT album_id FROM album_credits WHERE artist_id=artists.id
			UNION
			SELECT tracks.album_id FROM track_credits
				JOIN tracks ON tracks.id=track_credits.track_id
				WHERE track_credits.artist_id=artists.id
		)
		ORDER BY albums.tag_year DESC, albums.id
		LIMIT 1)
	) cover_album_id`), "artists.id", keys, func(c *artistCover) int { return c.ID })
	if err != nil {
		return nil, fmt.Errorf("load artist covers: %w", err)
	}

	byID := make(map[int]artistExtras, len(keys))
	for _, key := range keys {
		x := artistExtras{
			star:          stars[key],
			rating:        ratings[key],
			info:          infos[key],
			averageRating: averageRatings[key],
			roles:         roles[key],
			albumCount:    albumCounts[key],
		}
		if cover := covers[key]; cover != nil {
			x.coverAlbumID = cover.CoverAlbumID
		}
		byID[key] = x
	}
	return byID, nil
}

// loadArtistRoles collects the roles an artist is credited under, on albums
// and on tracks alike.
func loadArtistRoles(dbc *db.DB, keys []int) (map[int][]string, error) {
	type artistRole struct {
		ArtistID int
		Role     string
	}
	byID := make(map[int][]string, len(keys))
	for chunk := range db.ChunkIDs(keys) {
		var roles []artistRole
		if err := dbc.
			Raw(`SELECT artist_id, role FROM album_credits WHERE artist_id IN (?) UNION SELECT artist_id, role FROM track_credits WHERE artist_id IN (?) ORDER BY role`, chunk, chunk).
			Scan(&roles).Error; err != nil {
			return nil, err
		}
		for _, role := range roles {
			byID[role.ArtistID] = append(byID[role.ArtistID], role.Role)
		}
	}
	return byID, nil
}

// loadAlbumExtras gathers what the browse-by-tags album renderer reads. The
// browse-by-folder renderers read less and ask for the parts they need.
func loadAlbumExtras(r Render, albums []*db.Album) (map[int]albumExtras, error) {
	keys := ids(albums, func(a *db.Album) int { return a.ID })

	byID, err := loadAlbumUserExtras(r, keys)
	if err != nil {
		return nil, err
	}
	trackStats, err := loadAlbumTrackStats(r.DB, keys)
	if err != nil {
		return nil, fmt.Errorf("load track stats: %w", err)
	}
	plays, err := loadAlbumPlays(r.DB, r.UserID, keys)
	if err != nil {
		return nil, fmt.Errorf("load plays: %w", err)
	}
	credits, err := loadAlbumCredits(r.DB, keys, db.RoleAlbumArtist)
	if err != nil {
		return nil, fmt.Errorf("load credits: %w", err)
	}
	genres, err := loadGenres(r.DB, "album_genres", "album_id", keys)
	if err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}
	labels, err := db.FindAllByID(r.DB.DB, "album_id", keys, func(l *db.AlbumLabel) int { return l.AlbumID })
	if err != nil {
		return nil, fmt.Errorf("load labels: %w", err)
	}
	discTitles, err := db.FindAllByID(r.DB.DB, "album_id", keys, func(d *db.AlbumDiscTitle) int { return d.AlbumID })
	if err != nil {
		return nil, fmt.Errorf("load disc titles: %w", err)
	}

	for _, key := range keys {
		x := byID[key]
		x.credits, x.genres, x.labels, x.discTitles = credits[key], genres[key], labels[key], discTitles[key]
		x.withTrackStats(trackStats[key], plays[key])
		byID[key] = x
	}
	return byID, nil
}

// loadAlbumUserExtras gathers what the user has done with an album, which every
// album renderer shows.
func loadAlbumUserExtras(r Render, keys []int) (map[int]albumExtras, error) {
	mine := r.DB.Where("user_id=?", r.UserID)

	stars, err := db.FindByID(mine, "album_id", keys, func(s *db.AlbumStar) int { return s.AlbumID })
	if err != nil {
		return nil, fmt.Errorf("load album stars: %w", err)
	}
	ratings, err := db.FindByID(mine, "album_id", keys, func(r *db.AlbumRating) int { return r.AlbumID })
	if err != nil {
		return nil, fmt.Errorf("load album ratings: %w", err)
	}
	averageRatings, err := loadAverageRatings(r.DB, db.AlbumRating{}, "album_id", keys)
	if err != nil {
		return nil, fmt.Errorf("load album average ratings: %w", err)
	}

	byID := make(map[int]albumExtras, len(keys))
	for _, key := range keys {
		byID[key] = albumExtras{star: stars[key], rating: ratings[key], averageRating: averageRatings[key]}
	}
	return byID, nil
}

// withTrackStats takes the stats an album adds up over its tracks, which are
// absent for a folder row that has none.
func (x *albumExtras) withTrackStats(stats *albumTrackStats, play *albumPlay) {
	if stats != nil {
		x.trackCount, x.duration = stats.TrackCount, stats.Duration
	}
	if play != nil {
		x.playCount, x.playTime = play.PlayCount, play.PlayTime
	}
}

type albumTrackStats struct {
	ID         int
	TrackCount int
	Duration   int
}

// loadAlbumTrackStats counts what an album adds up to over its tracks. Only the
// renderers that show a song count ask for it: a folder row standing for a
// directory has no tracks of its own and shows its child albums instead.
func loadAlbumTrackStats(dbc *db.DB, keys []int) (map[int]*albumTrackStats, error) {
	return scanByID(dbc.Model(db.Track{}).Select("album_id AS id, count(id) track_count, sum(length) duration").Group("album_id"),
		"album_id", keys, func(s *albumTrackStats) int { return s.ID })
}

type albumPlay struct {
	ID        int
	PlayCount float64
	PlayTime  db.ScanTime
}

// loadAlbumPlays sums the user's plays over an album's tracks.
func loadAlbumPlays(dbc *db.DB, userID int, keys []int) (map[int]*albumPlay, error) {
	byID := make(map[int]*albumPlay, len(keys))
	for chunk := range db.ChunkIDs(keys) {
		var plays []*albumPlay
		if err := dbc.
			Raw(`SELECT t.album_id AS id, sum(track_plays.count) play_count, max(track_plays.time) play_time
				FROM track_plays
				JOIN tracks t ON t.id=track_plays.track_id
				WHERE track_plays.user_id=? AND t.album_id IN (?)
				GROUP BY t.album_id`, userID, chunk).
			Scan(&plays).Error; err != nil {
			return nil, err
		}
		for _, play := range plays {
			byID[play.ID] = play
		}
	}
	return byID, nil
}

// loadAlbumChildCounts counts the albums directly below each folder row.
func loadAlbumChildCounts(dbc *db.DB, keys []int) (map[int]int, error) {
	return scanCounts(dbc.Model(db.Album{}).Select("parent_id AS id, count(id) count").Group("parent_id"),
		"parent_id", keys)
}

// loadTrackExtras gathers what both track renderers read. They differ only in
// which credits they name, so that is the one thing passed in.
func loadTrackExtras(r Render, tracks []*db.Track, roles []string) (map[int]trackExtras, error) {
	keys := ids(tracks, func(t *db.Track) int { return t.ID })
	mine := r.DB.Where("user_id=?", r.UserID)

	stars, err := db.FindByID(mine, "track_id", keys, func(s *db.TrackStar) int { return s.TrackID })
	if err != nil {
		return nil, fmt.Errorf("load stars: %w", err)
	}
	ratings, err := db.FindByID(mine, "track_id", keys, func(r *db.TrackRating) int { return r.TrackID })
	if err != nil {
		return nil, fmt.Errorf("load ratings: %w", err)
	}
	plays, err := db.FindByID(mine, "track_id", keys, func(p *db.TrackPlay) int { return p.TrackID })
	if err != nil {
		return nil, fmt.Errorf("load plays: %w", err)
	}
	averageRatings, err := loadAverageRatings(r.DB, db.TrackRating{}, "track_id", keys)
	if err != nil {
		return nil, fmt.Errorf("load average ratings: %w", err)
	}
	credits, err := loadTrackCredits(r.DB, keys, roles...)
	if err != nil {
		return nil, fmt.Errorf("load credits: %w", err)
	}
	genres, err := loadGenres(r.DB, "track_genres", "track_id", keys)
	if err != nil {
		return nil, fmt.Errorf("load genres: %w", err)
	}
	isrcs, err := db.FindAllByID(r.DB.DB, "track_id", keys, func(i *db.TrackISRC) int { return i.TrackID })
	if err != nil {
		return nil, fmt.Errorf("load isrcs: %w", err)
	}

	albumKeys := ids(tracks, func(t *db.Track) int { return t.AlbumID })
	albums, err := db.FindByID(r.DB.DB, "id", albumKeys, func(a *db.Album) int { return a.ID })
	if err != nil {
		return nil, fmt.Errorf("load albums: %w", err)
	}
	byID := make(map[int]trackExtras, len(keys))
	for _, track := range tracks {
		byID[track.ID] = trackExtras{
			album:         albums[track.AlbumID],
			star:          stars[track.ID],
			rating:        ratings[track.ID],
			play:          plays[track.ID],
			averageRating: averageRatings[track.ID],
			credits:       credits[track.ID],
			genres:        genres[track.ID],
			isrcs:         isrcs[track.ID],
		}
	}
	return byID, nil
}

// Credits

// loadTrackCredits collects a track's credits with the artist each one names.
// Passing no roles takes them all.
func loadTrackCredits(dbc *db.DB, keys []int, roles ...string) (map[int][]*db.TrackCredit, error) {
	q := dbc.DB
	if len(roles) > 0 {
		q = q.Where("role IN (?)", roles)
	}
	byID, err := db.FindAllByID(q, "track_id", keys, func(c *db.TrackCredit) int { return c.TrackID })
	if err != nil {
		return nil, err
	}
	return byID, fillCreditArtists(dbc, byID,
		func(c *db.TrackCredit) int { return c.ArtistID },
		func(c *db.TrackCredit, a *db.Artist) { c.Artist = a })
}

// loadAlbumCredits collects an album's credits with the artist each one names.
func loadAlbumCredits(dbc *db.DB, keys []int, role string) (map[int][]*db.AlbumCredit, error) {
	byID, err := db.FindAllByID(dbc.Where("role=?", role), "album_id", keys, func(c *db.AlbumCredit) int { return c.AlbumID })
	if err != nil {
		return nil, err
	}
	return byID, fillCreditArtists(dbc, byID,
		func(c *db.AlbumCredit) int { return c.ArtistID },
		func(c *db.AlbumCredit, a *db.Artist) { c.Artist = a })
}

// fillCreditArtists names the artist each credit points at, fetching every
// artist once however many credits share them.
func fillCreditArtists[C any](dbc *db.DB, byID map[int][]*C, artistID func(*C) int, set func(*C, *db.Artist)) error {
	var credits []*C
	for _, forParent := range byID {
		credits = append(credits, forParent...)
	}
	artists, err := db.FindByID(dbc.DB, "id", ids(credits, artistID), func(a *db.Artist) int { return a.ID })
	if err != nil {
		return err
	}
	for _, credit := range credits {
		if artist, ok := artists[artistID(credit)]; ok {
			set(credit, artist)
		}
	}
	return nil
}

// Genres

// loadGenres reads a many2many genre relation. The join table is read first so
// the genres themselves are fetched once however many rows share them.
func loadGenres(dbc *db.DB, table, fk string, keys []int) (map[int][]*db.Genre, error) {
	type link struct {
		ParentID int
		GenreID  int
	}
	var links []link
	for chunk := range db.ChunkIDs(keys) {
		var found []link
		if err := dbc.
			Table(table).
			Select(fk+" parent_id, genre_id").
			Where(fk+" IN (?)", chunk).
			Order("genre_id").
			Scan(&found).Error; err != nil {
			return nil, err
		}
		links = append(links, found...)
	}

	genreIDs := make([]int, 0, len(links))
	for _, l := range links {
		genreIDs = append(genreIDs, l.GenreID)
	}
	genres, err := db.FindByID(dbc.DB, "id", genreIDs, func(g *db.Genre) int { return g.ID })
	if err != nil {
		return nil, err
	}

	byID := make(map[int][]*db.Genre, len(keys))
	for _, l := range links {
		if genre, ok := genres[l.GenreID]; ok {
			byID[l.ParentID] = append(byID[l.ParentID], genre)
		}
	}
	return byID, nil
}

// loadGenreCounts counts how much of a join table each genre covers. It asks per
// genre rather than grouping the whole join table, which is far slower.
func loadGenreCounts(dbc *db.DB, table string, keys []int) (map[int]int, error) {
	return scanCounts(dbc.
		Model(db.Genre{}).
		Select(fmt.Sprintf("genres.id AS id, (SELECT count(1) FROM %s WHERE genre_id=genres.id) count", table)),
		"genres.id", keys)
}

// Scopes

// AlbumWithUserPlay joins the user's play stats purely so queries can order and
// filter on play_count, play_length, and play_time. Nothing reads the joined
// columns back: what the user sees comes from loadAlbumPlays.
func AlbumWithUserPlay(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Joins(`LEFT JOIN (
				SELECT t.album_id,
					sum(track_plays.count) play_count,
					sum(track_plays.length) play_length,
					max(track_plays.time) play_time
				FROM track_plays
				JOIN tracks t ON t.id=track_plays.track_id
				WHERE track_plays.user_id=?
				GROUP BY t.album_id
			) album_plays ON album_plays.album_id=albums.id`, userID)
	}
}

func WithAlbumRootDir(rootDir string) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		if rootDir == "" {
			return q
		}
		return q.Where("albums.root_dir=?", rootDir)
	}
}

// Loading by id

func ids[T any](rows []*T, id func(*T) int) []int {
	out := make([]int, 0, len(rows))
	for _, row := range rows {
		out = append(out, id(row))
	}
	return out
}

// scanByID runs an aggregate keyed by an id column and returns it keyed by that
// id. Aggregates need Scan rather than Find, since they select computed columns
// rather than a table's own. Callers group where the aggregate needs it.
func scanByID[T any](q *gorm.DB, column string, keys []int, id func(*T) int) (map[int]*T, error) {
	byID := make(map[int]*T, len(keys))
	for chunk := range db.ChunkIDs(keys) {
		var found []*T
		if err := q.Where(column+" IN (?)", chunk).Scan(&found).Error; err != nil {
			return nil, err
		}
		for _, row := range found {
			byID[id(row)] = row
		}
	}
	return byID, nil
}

// scanCounts runs an aggregate selecting one count per id.
func scanCounts(q *gorm.DB, column string, keys []int) (map[int]int, error) {
	type idCount struct {
		ID    int
		Count int
	}
	rows, err := scanByID(q, column, keys, func(c *idCount) int { return c.ID })
	if err != nil {
		return nil, err
	}
	byID := make(map[int]int, len(rows))
	for id, row := range rows {
		byID[id] = row.Count
	}
	return byID, nil
}

// loadAverageRatings averages every user's rating of a row. The id column is
// aliased so one query serves artists, albums, and tracks.
func loadAverageRatings(dbc *db.DB, model any, column string, keys []int) (map[int]float64, error) {
	type averageRating struct {
		ID            int
		AverageRating float64
	}
	rows, err := scanByID(dbc.
		Model(model).
		Select(fmt.Sprintf("%s AS id, cast(avg(rating)*100 AS INT)/100.0 average_rating", column)).
		Group(column),
		column, keys, func(r *averageRating) int { return r.ID })
	if err != nil {
		return nil, err
	}
	byID := make(map[int]float64, len(rows))
	for id, row := range rows {
		byID[id] = row.AverageRating
	}
	return byID, nil
}
