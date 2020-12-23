package db

import (
	"errors"
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// $ date '+%Y%m%d%H%M'

func migrateInitSchema() gormigrate.Migration {
	return gormigrate.Migration{
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
			)
		},
	}
}

func migrateCreateInitUser() gormigrate.Migration {
	return gormigrate.Migration{
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
			if !errors.Is(err, gorm.ErrRecordNotFound) {
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
}

func migrateMergePlaylist() gormigrate.Migration {
	return gormigrate.Migration{
		ID: "202002192222",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("playlist_items") {
				return nil
			}

			err := tx.Exec(`
				UPDATE playlists
				SET items=( SELECT group_concat(track_id) FROM (
					SELECT track_id
					FROM playlist_items
					WHERE playlist_items.playlist_id=playlists.id
					ORDER BY created_at
				) );`,
			).
				Error
			if err != nil {
				return fmt.Errorf("step migrate: %w", err)
			}

			err = tx.Migrator().DropTable("playlist_items")
			if err != nil {
				return fmt.Errorf("step drop: %w", err)
			}

			return nil
		},
	}
}

func migrateCreateTranscode() gormigrate.Migration {
	return gormigrate.Migration{
		ID: "202003111222",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				TranscodePreference{},
			)
		},
	}
}

func migrateAddGenre() gormigrate.Migration {
	return gormigrate.Migration{
		ID: "202003121330",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				Genre{},
				Album{},
				Track{},
			)
		},
	}
}

func migrateUpdateTranscodePrefIDX() gormigrate.Migration {
	return gormigrate.Migration{
		ID: "202003241509",
		Migrate: func(tx *gorm.DB) error {
			if tx.Migrator().HasIndex(&TranscodePreference{}, "idx_user_id_client") {
				return nil
			}

			err := tx.Migrator().RenameTable("transcode_preferences", "transcode_preferences_orig")
			if err != nil {
				return fmt.Errorf("step rename: %w", err)
			}

			err = tx.AutoMigrate(
				TranscodePreference{},
			)
			if err != nil {
				return fmt.Errorf("step create: %w", err)
			}

			step := tx.Exec(`
				INSERT INTO transcode_preferences (user_id, client, profile)
					SELECT user_id, client, profile
					FROM transcode_preferences_orig;
			`)
			if err := step.Error; err != nil {
				return fmt.Errorf("step copy: %w", err)
			}

			err = tx.Migrator().DropTable("transcode_preferences_orig")
			if err != nil {
				return fmt.Errorf("step drop orig: %w", err)
			}
			return nil
		},
	}
}

func migrateAddAlbumIDX() gormigrate.Migration {
	return gormigrate.Migration{
		ID: "202004302006",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				Album{},
			)
		},
	}
}

func migrateMultiGenre() gormigrate.Migration {
	return gormigrate.Migration{
		ID: "202012151806",
		Migrate: func(tx *gorm.DB) error {
			err := tx.AutoMigrate(
				Track{},
				Album{},
				Genre{},
				TrackGenre{},
				AlbumGenre{},
			)
			if err != nil {
				return fmt.Errorf("step auto migrate: %w", err)
			}

			var genreCount int64
			tx.
				Model(Genre{}).
				Count(&genreCount)
			if genreCount == 0 {
				return nil
			}

			step := tx.Exec(`
				INSERT INTO track_genres (track_id, genre_id)
					SELECT id, tag_genre_id
					FROM tracks
					WHERE tag_genre_id IS NOT NULL;
			`)
			if err := step.Error; err != nil {
				return fmt.Errorf("step migrate track genres: %w", err)
			}

			step = tx.Exec(`
				UPDATE tracks SET tag_genre_id=NULL;
			`)
			if err := step.Error; err != nil {
				return fmt.Errorf("step set tracks tag_genre_id null: %w", err)
			}

			step = tx.Exec(`
				INSERT INTO album_genres (album_id, genre_id)
					SELECT id, tag_genre_id
					FROM albums
					WHERE tag_genre_id IS NOT NULL;
			`)
			if err := step.Error; err != nil {
				return fmt.Errorf("step migrate album genres: %w", err)
			}

			step = tx.Exec(`
				UPDATE albums SET tag_genre_id=NULL;
			`)
			if err := step.Error; err != nil {
				return fmt.Errorf("step set albums tag_genre_id null: %w", err)
			}
			return nil
		},
	}
}
