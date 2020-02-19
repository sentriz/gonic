//nolint:deadcode,varcheck
package db

import (
	"github.com/jinzhu/gorm"
	"gopkg.in/gormigrate.v1"
)

var migrationCreateInitUser = gormigrate.Migration{
	ID: "202002192019",
	Migrate: func(tx *gorm.DB) error {
		const (
			initUsername = "admin"
			initPassword = "admin"
		)
		err := tx.
			Where("name = ?", initUsername).
			First(&User{}).
			Error
		if gorm.IsRecordNotFoundError(err) {
			tx.Create(&User{
				Name:     initUsername,
				Password: initPassword,
				IsAdmin:  true,
			})
			return nil
		}
		return err
	},
}

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

var migrationMergePlaylist = gormigrate.Migration{
	ID: "202002192222",
	Migrate: func(tx *gorm.DB) error {
		return tx.Exec(`
			UPDATE playlists
			SET items = (
				SELECT group_concat(track_id) FROM (
					SELECT track_id
					FROM playlist_items
					WHERE playlist_items.playlist_id=playlists.id
					ORDER BY created_at
				)
			);
			DROP TABLE playlist_items;`,
		).
			Error
	},
}
