//nolint:goerr113
package db

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/jinzhu/gorm"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"gopkg.in/gormigrate.v1"
)

type MigrationContext struct {
	OriginalMusicPath string
	PlaylistsPath     string
	PodcastsPath      string
}

func (db *DB) Migrate(ctx MigrationContext) error {
	options := &gormigrate.Options{
		TableName:      "migrations",
		IDColumnName:   "id",
		IDColumnSize:   255,
		UseTransaction: false,
	}

	// $ date '+%Y%m%d%H%M'
	migrations := []*gormigrate.Migration{
		construct(ctx, "202002192100", migrateInitSchema),
		construct(ctx, "202002192019", migrateCreateInitUser),
		construct(ctx, "202002192222", migrateMergePlaylist),
		construct(ctx, "202003111222", migrateCreateTranscode),
		construct(ctx, "202003121330", migrateAddGenre),
		construct(ctx, "202003241509", migrateUpdateTranscodePrefIDX),
		construct(ctx, "202004302006", migrateAddAlbumIDX),
		construct(ctx, "202012151806", migrateMultiGenre),
		construct(ctx, "202101081149", migrateListenBrainz),
		construct(ctx, "202101111537", migratePodcast),
		construct(ctx, "202102032210", migrateBookmarks),
		construct(ctx, "202102191448", migratePodcastAutoDownload),
		construct(ctx, "202110041330", migrateAlbumCreatedAt),
		construct(ctx, "202111021951", migrateAlbumRootDir),
		construct(ctx, "202201042236", migrateArtistGuessedFolder),
		construct(ctx, "202202092013", migrateArtistCover),
		construct(ctx, "202202121809", migrateAlbumRootDirAgain),
		construct(ctx, "202202241218", migratePublicPlaylist),
		construct(ctx, "202204270903", migratePodcastDropUserID),
		construct(ctx, "202206011628", migrateInternetRadioStations),
		construct(ctx, "202206101425", migrateUser),
		construct(ctx, "202207251148", migrateStarRating),
		construct(ctx, "202211111057", migratePlaylistsQueuesToFullID),
		construct(ctx, "202304221528", migratePlaylistsToM3U),
	}

	return gormigrate.
		New(db.DB, options, migrations).
		Migrate()
}

func construct(ctx MigrationContext, id string, f func(*gorm.DB, MigrationContext) error) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: id,
		Migrate: func(db *gorm.DB) error {
			tx := db.Begin()
			defer tx.Commit()
			if err := f(tx, ctx); err != nil {
				return fmt.Errorf("%q: %w", id, err)
			}
			log.Printf("migration '%s' finished", id)
			return nil
		},
		Rollback: func(*gorm.DB) error {
			return nil
		},
	}
}

func migrateInitSchema(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Genre{},
		TrackGenre{},
		AlbumGenre{},
		Track{},
		Artist{},
		User{},
		Setting{},
		Play{},
		Album{},
		PlayQueue{},
	).
		Error
}

func migrateCreateInitUser(tx *gorm.DB, _ MigrationContext) error {
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
}

func migrateMergePlaylist(tx *gorm.DB, _ MigrationContext) error {
	if !tx.HasTable("playlists") {
		return nil
	}
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
}

func migrateCreateTranscode(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		TranscodePreference{},
	).
		Error
}

func migrateAddGenre(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Genre{},
		Album{},
		Track{},
	).
		Error
}

func migrateUpdateTranscodePrefIDX(tx *gorm.DB, _ MigrationContext) error {
	var hasIDX int
	tx.
		Select("1").
		Table("sqlite_master").
		Where("type = ?", "index").
		Where("name = ?", "idx_user_id_client").
		Count(&hasIDX)
	if hasIDX == 1 {
		// index already exists
		return nil
	}

	step := tx.Exec(`
		ALTER TABLE transcode_preferences RENAME TO transcode_preferences_orig;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step rename: %w", err)
	}

	step = tx.AutoMigrate(
		TranscodePreference{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step create: %w", err)
	}

	step = tx.Exec(`
		INSERT INTO transcode_preferences (user_id, client, profile)
			SELECT user_id, client, profile
			FROM transcode_preferences_orig;
		DROP TABLE transcode_preferences_orig;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step copy: %w", err)
	}
	return nil
}

func migrateAddAlbumIDX(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Album{},
	).
		Error
}

func migrateMultiGenre(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(
		Genre{},
		TrackGenre{},
		AlbumGenre{},
		Track{},
		Album{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	var genreCount int
	tx.
		Model(Genre{}).
		Count(&genreCount)
	if genreCount == 0 {
		return nil
	}

	step = tx.Exec(`
		INSERT INTO track_genres (track_id, genre_id)
			SELECT id, tag_genre_id
			FROM tracks
			WHERE tag_genre_id IS NOT NULL;
		UPDATE tracks SET tag_genre_id=NULL;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate track genres: %w", err)
	}

	step = tx.Exec(`
		INSERT INTO album_genres (album_id, genre_id)
			SELECT id, tag_genre_id
			FROM albums
			WHERE tag_genre_id IS NOT NULL;
		UPDATE albums SET tag_genre_id=NULL;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate album genres: %w", err)
	}
	return nil
}

func migrateListenBrainz(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(
		User{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}
	return nil
}

func migratePodcast(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Podcast{},
		PodcastEpisode{},
	).
		Error
}

func migrateBookmarks(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Bookmark{},
	).
		Error
}

func migratePodcastAutoDownload(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Podcast{},
	).
		Error
}

func migrateAlbumCreatedAt(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(
		Album{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}
	step = tx.Exec(`
		UPDATE albums SET created_at=modified_at;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate album created_at: %w", err)
	}
	return nil
}

func migrateAlbumRootDir(tx *gorm.DB, ctx MigrationContext) error {
	step := tx.AutoMigrate(
		Album{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}
	step = tx.Exec(`
		DROP INDEX IF EXISTS idx_left_path_right_path;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop idx: %w", err)
	}

	step = tx.Exec(`
		UPDATE albums SET root_dir=? WHERE root_dir IS NULL
	`, ctx.OriginalMusicPath)
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop idx: %w", err)
	}
	return nil
}

func migrateArtistGuessedFolder(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Artist{}).Error
}

func migrateArtistCover(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(
		Artist{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	if !tx.Dialect().HasColumn("artists", "guessed_folder_id") {
		return nil
	}

	step = tx.Exec(`
		ALTER TABLE artists DROP COLUMN guessed_folder_id
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop column: %w", err)
	}
	return nil
}

// there was an issue with that migration, try it again since it's updated
func migrateAlbumRootDirAgain(tx *gorm.DB, ctx MigrationContext) error {
	return migrateAlbumRootDir(tx, ctx)
}

func migratePublicPlaylist(tx *gorm.DB, _ MigrationContext) error {
	if !tx.HasTable("playlists") {
		return nil
	}
	return tx.AutoMigrate(_OldPlaylist{}).Error
}

func migratePodcastDropUserID(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(
		Podcast{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	if !tx.Dialect().HasColumn("podcasts", "user_id") {
		return nil
	}

	step = tx.Exec(`
		ALTER TABLE podcasts DROP COLUMN user_id;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate podcasts drop user_id: %w", err)
	}
	return nil
}

func migrateInternetRadioStations(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		InternetRadioStation{},
	).
		Error
}

func migrateUser(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		User{},
	).
		Error
}

func migrateStarRating(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Album{},
		AlbumStar{},
		AlbumRating{},
		Artist{},
		ArtistStar{},
		ArtistRating{},
		Track{},
		TrackStar{},
		TrackRating{},
	).
		Error
}

func migratePlaylistsQueuesToFullID(tx *gorm.DB, _ MigrationContext) error {
	if !tx.HasTable("playlists") {
		return nil
	}

	step := tx.Exec(`
		UPDATE playlists SET items=('tr-' || items) WHERE items IS NOT NULL;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate playlists to full id: %w", err)
	}
	step = tx.Exec(`
		UPDATE playlists SET items=REPLACE(items,',',',tr-') WHERE items IS NOT NULL;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate playlists to full id: %w", err)
	}

	step = tx.Exec(`
		UPDATE play_queues SET items=('tr-' || items) WHERE items IS NOT NULL;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate play_queues to full id: %w", err)
	}
	step = tx.Exec(`
		UPDATE play_queues SET items=REPLACE(items,',',',tr-') WHERE items IS NOT NULL;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate play_queues to full id: %w", err)
	}
	step = tx.Exec(`
		ALTER TABLE play_queues ADD COLUMN newcurrent varchar[255];
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate play_queues to full id: %w", err)
	}
	step = tx.Exec(`
		UPDATE play_queues SET newcurrent=('tr-' || CAST(current AS varchar(10)));
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate play_queues to full id: %w", err)
	}
	step = tx.Exec(`
		ALTER TABLE play_queues DROP COLUMN current;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate play_queues to full id: %w", err)
	}
	step = tx.Exec(`
		ALTER TABLE play_queues RENAME COLUMN newcurrent TO "current";
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step migrate play_queues to full id: %w", err)
	}

	return nil
}

func migratePlaylistsToM3U(tx *gorm.DB, ctx MigrationContext) error {
	if ctx.PlaylistsPath == "" || !tx.HasTable("playlists") {
		return nil
	}

	// local copy of specidpaths.Locate to avoid circular dep
	locate := func(id specid.ID) string {
		switch id.Type {
		case specid.Track:
			var track Track
			tx.Preload("Album").Where("id=?", id.Value).Find(&track)
			return track.AbsPath()
		case specid.PodcastEpisode:
			var pe PodcastEpisode
			tx.Where("id=?", id.Value).Find(&pe)
			if pe.Path == "" {
				return ""
			}
			return filepath.Join(ctx.PodcastsPath, pe.Path)
		}
		return ""
	}

	store, err := playlist.NewStore(ctx.PlaylistsPath)
	if err != nil {
		return fmt.Errorf("create playlists store: %w", err)
	}

	var prevs []*_OldPlaylist
	if err := tx.Find(&prevs).Error; err != nil {
		return fmt.Errorf("fetch old playlists: %w", err)
	}

	for _, prev := range prevs {
		var pl playlist.Playlist
		pl.UpdatedAt = time.Now()
		pl.UserID = prev.UserID
		pl.Name = prev.Name
		pl.Comment = prev.Comment
		pl.IsPublic = prev.IsPublic

		for _, id := range splitIDs(prev.Items, ",") {
			path := locate(id)
			if path == "" {
				log.Printf("migrating: can't find item %s from playlist %q on filesystem", id, prev.Name)
				continue
			}
			pl.Items = append(pl.Items, path)
		}

		if err := store.Write(playlist.NewPath(prev.UserID, prev.Name), &pl); err != nil {
			return fmt.Errorf("write playlist: %w", err)
		}
	}

	return nil
}
