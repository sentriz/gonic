package spec

import (
	"strings"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
)

// Warm avoids a gorm v1 race that can poison the ModelStruct cache for these
// embedded types under concurrent first-time access.
func Warm(dbc *gorm.DB) {
	for _, v := range []any{&ArtistRow{}, &AlbumRow{}, &GenreRow{}} {
		dbc.NewScope(v).GetModelStruct()
	}
}

type ArtistRow struct {
	db.Artist
	AlbumCount int
	Roles      string
}

func (ArtistRow) TableName() string { return "artists" }

func (a *ArtistRow) GetRoles() []string {
	if a.Roles == "" {
		return nil
	}
	return strings.Split(a.Roles, ",")
}

const artistRolesColumn = `(
	SELECT GROUP_CONCAT(role) FROM (
		SELECT role FROM album_credits WHERE artist_id=artists.id
		UNION
		SELECT role FROM track_credits WHERE artist_id=artists.id
	)
) roles`

func ArtistWithRoles(q *gorm.DB) *gorm.DB {
	return q.Select("artists.*, " + artistRolesColumn)
}

func ArtistWithRolesAndAlbumCount(q *gorm.DB) *gorm.DB {
	return q.
		Select("artists.*, count(album_credits.album_id) album_count, " + artistRolesColumn).
		Group("artists.id")
}

type AlbumRow struct {
	db.Album
	ChildCount int
	Duration   int
}

func (AlbumRow) TableName() string { return "albums" }

func AlbumWithTrackCounts(q *gorm.DB) *gorm.DB {
	return q.
		Select("albums.*, count(tracks.id) child_count, sum(tracks.length) duration").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Group("albums.id")
}

func AlbumWithChildAlbumCounts(q *gorm.DB) *gorm.DB {
	return q.
		Select("albums.*, count(sub.id) child_count").
		Joins("LEFT JOIN albums sub ON albums.id=sub.parent_id").
		Group("albums.id")
}

type GenreRow struct {
	db.Genre
	AlbumCount int
	TrackCount int
}

func (GenreRow) TableName() string { return "genres" }

func GenreWithCounts(q *gorm.DB) *gorm.DB {
	return q.Select(`genres.*,
		(SELECT count(1) FROM album_genres WHERE genre_id=genres.id) album_count,
		(SELECT count(1) FROM track_genres WHERE genre_id=genres.id) track_count`).
		Group("genres.id")
}

func AlbumWithAlbumArtistCredits(q *gorm.DB) *gorm.DB {
	return q.Preload("Credits", func(q *gorm.DB) *gorm.DB {
		return q.Where("role=?", db.RoleAlbumArtist).Preload("Artist")
	})
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

func AlbumWithUserData(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Preload("AlbumStar", "user_id=?", userID).
			Preload("AlbumRating", "user_id=?", userID)
	}
}

func ArtistWithUserData(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Preload("ArtistStar", "user_id=?", userID).
			Preload("ArtistRating", "user_id=?", userID)
	}
}

func TrackWithUserData(userID int) func(*gorm.DB) *gorm.DB {
	return func(q *gorm.DB) *gorm.DB {
		return q.
			Preload("TrackStar", "user_id=?", userID).
			Preload("TrackRating", "user_id=?", userID)
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
