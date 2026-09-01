//nolint:goconst
package spec

import (
	"strings"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
)

// Warm avoids a gorm v1 race that can poison the ModelStruct cache for these
// embedded types under concurrent first-time access.
func Warm(dbc *gorm.DB) {
	for _, v := range []any{&ArtistRow{}, &AlbumRow{}, &TrackRow{}, &GenreRow{}} {
		dbc.NewScope(v).GetModelStruct()
	}
}

// Artist

type ArtistRow struct {
	db.Artist
	AlbumCount    int
	Roles         string
	AverageRating float64
	CoverAlbumID  int
}

func (ArtistRow) TableName() string { return "artists" }

func (a *ArtistRow) GetRoles() []string {
	if a.Roles == "" {
		return nil
	}
	return strings.Split(a.Roles, ",")
}

const artistAverageRatingColumn = `(SELECT cast(coalesce(avg(rating), 0)*100 AS INT)/100.0 FROM artist_ratings WHERE artist_id=artists.id) average_rating`

const artistRolesColumn = `(
	SELECT GROUP_CONCAT(role) FROM (
		SELECT role FROM album_credits WHERE artist_id=artists.id
		UNION
		SELECT role FROM track_credits WHERE artist_id=artists.id
	)
) roles`

// cover_album_id is a fallback cover for artists with no lastfm image: prefer their most recent
// album-artist album that has a cover, else any album they're credited on. the album-artist arm
// short-circuits the coalesce so it stays cheap for the (album-artist only) browse lists
const artistCoverAlbumIDColumn = `coalesce(
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
) cover_album_id`

func ArtistWithRoles(q *gorm.DB) *gorm.DB {
	return q.Select([]string{"artists.*", artistAverageRatingColumn, artistRolesColumn, artistCoverAlbumIDColumn})
}

func ArtistWithRolesAndAlbumCount(q *gorm.DB) *gorm.DB {
	return q.
		Select([]string{"artists.*", "count(album_credits.album_id) album_count", artistAverageRatingColumn, artistRolesColumn, artistCoverAlbumIDColumn}).
		Group("artists.id")
}

func ArtistWithUserData(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Preload("ArtistStar", "user_id=?", userID).
			Preload("ArtistRating", "user_id=?", userID)
	}
}

// Album

type AlbumRow struct {
	db.Album
	ChildCount    int
	Duration      int
	PlayCount     float64
	PlayTime      db.ScanTime
	AverageRating float64
}

func (AlbumRow) TableName() string { return "albums" }

const albumAverageRatingColumn = `(SELECT cast(coalesce(avg(rating), 0)*100 AS INT)/100.0 FROM album_ratings WHERE album_id=albums.id) average_rating`

func AlbumWithUserPlay(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Select([]string{
				"albums.*",
				"count(tracks.id) child_count",
				"sum(tracks.length) duration",
				"coalesce(album_plays.play_count, 0) play_count",
				"album_plays.play_length play_length",
				"album_plays.play_time play_time",
				albumAverageRatingColumn,
			}).
			Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
			Joins(`LEFT JOIN (
				SELECT t.album_id,
					sum(track_plays.count) play_count,
					sum(track_plays.length) play_length,
					max(track_plays.time) play_time
				FROM track_plays
				JOIN tracks t ON t.id=track_plays.track_id
				WHERE track_plays.user_id=?
				GROUP BY t.album_id
			) album_plays ON album_plays.album_id=albums.id`, userID).
			Group("albums.id")
	}
}

func AlbumWithChildAlbumCounts(q *gorm.DB) *gorm.DB {
	return q.
		Select([]string{"albums.*", "count(sub.id) child_count", albumAverageRatingColumn}).
		Joins("LEFT JOIN albums sub ON albums.id=sub.parent_id").
		Group("albums.id")
}

func AlbumWithAlbumArtistCredits(q *gorm.DB) *gorm.DB {
	return q.Preload("Credits", func(q *gorm.DB) *gorm.DB {
		return q.Where("role=?", db.RoleAlbumArtist).Preload("Artist")
	})
}

func AlbumWithUserData(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Preload("AlbumStar", "user_id=?", userID).
			Preload("AlbumRating", "user_id=?", userID)
	}
}

// Track

type TrackRow struct {
	db.Track
	AverageRating float64
}

func (TrackRow) TableName() string { return "tracks" }

const trackAverageRatingColumn = `(SELECT cast(coalesce(avg(rating), 0)*100 AS INT)/100.0 FROM track_ratings WHERE track_id=tracks.id) average_rating`

func TrackWithAverageRating(q *gorm.DB) *gorm.DB {
	return q.Select([]string{"tracks.*", trackAverageRatingColumn})
}

func TrackWithArtistCredits(q *gorm.DB) *gorm.DB {
	return q.Preload("Credits", func(q *gorm.DB) *gorm.DB {
		return q.Where("role=?", db.RoleArtist).Preload("Artist")
	})
}

func TrackWithAlbumArtistCredits(q *gorm.DB) *gorm.DB {
	return q.Preload("Album.Credits", func(q *gorm.DB) *gorm.DB {
		return q.Where("role=?", db.RoleAlbumArtist).Preload("Artist")
	})
}

func TrackWithUserData(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Preload("TrackStar", "user_id=?", userID).
			Preload("TrackRating", "user_id=?", userID)
	}
}

// Genre

type GenreRow struct {
	db.Genre
	AlbumCount int
	TrackCount int
}

func (GenreRow) TableName() string { return "genres" }

func GenreWithCounts(q *gorm.DB) *gorm.DB {
	return q.Select([]string{
		"genres.*",
		"(SELECT count(1) FROM album_genres WHERE genre_id=genres.id) album_count",
		"(SELECT count(1) FROM track_genres WHERE genre_id=genres.id) track_count",
	}).
		Group("genres.id")
}

// Shared

func WithAlbumRootDir(rootDir string) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		if rootDir == "" {
			return q
		}
		return q.Where("albums.root_dir=?", rootDir)
	}
}
