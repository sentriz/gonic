//nolint:deadcode,varcheck
package db

import (
	"github.com/jinzhu/gorm"
	"gopkg.in/gormigrate.v1"
)

// $ date '+%Y%m%d%H%M'
// not really a migration
var migrationInitSchema = gormigrate.Migration{
	ID: "202002192100",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			Artist{},
			Track{},
			User{},
			Setting{},
			Play{},
			Album{},
			Playlist{},
			PlayQueue{},
		).
			Error
	},
}

// not really a migration
var migrationCreateInitUser = gormigrate.Migration{
	ID: "202002192019",
	Migrate: func(tx *gorm.DB) error {
		const (
			initUsername = "admin"
			initPassword = "admin"
		)
		err := tx.
			Where("name=?", initUsername).
			First(&User{}).
			Error
		if !gorm.IsRecordNotFoundError(err) {
			return nil
		}
		return tx.Create(&User{
			Name:     initUsername,
			Password: initPassword,
			IsAdmin:  true,
		}).
			Error
	},
}

var migrationMergePlaylist = gormigrate.Migration{
	ID: "202002192222",
	Migrate: func(tx *gorm.DB) error {
		if !tx.HasTable("playlist_items") {
			return nil
		}
		return tx.Exec(`
			UPDATE playlists
			SET items=( SELECT group_concat(track_id) FROM (
				SELECT track_id
				FROM playlist_items
				WHERE playlist_items.playlist_id=playlists.id
				ORDER BY created_at
			) );
			DROP TABLE playlist_items;`,
		).
			Error
	},
}

var migrationCreateTranscode = gormigrate.Migration{
	ID: "202003111222",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			TranscodePreference{},
		).
			Error
	},
}

var migrationAddGenre = gormigrate.Migration{
	ID: "202003121330",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			Genre{},
			Album{},
			Track{},
		).
			Error
	},
}
