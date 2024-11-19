//nolint:goerr113
package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"go.senan.xyz/gonic/fileutil"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/sandbox"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"gopkg.in/gormigrate.v1"
)

type MigrationContext struct {
	Production        bool
	DBPath            string
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
		constructNoTx(ctx, "202212272312", backupDBPre016),
		construct(ctx, "202304221528", migratePlaylistsToM3U),
		construct(ctx, "202305301718", migratePlayCountToLength),
		construct(ctx, "202307281628", migrateAlbumArtistsMany2Many),
		construct(ctx, "202309070009", migrateDeleteArtistCoverField),
		construct(ctx, "202309131743", migrateArtistInfo),
		construct(ctx, "202309161411", migratePlaylistsPaths),
		construct(ctx, "202310252205", migrateAlbumTagArtistString),
		construct(ctx, "202310281803", migrateTrackArtists),
		construct(ctx, "202311062259", migrateArtistAppearances),
		construct(ctx, "202311072309", migrateAlbumInfo),
		construct(ctx, "202311082304", migrateTemporaryDisplayAlbumArtist),
		construct(ctx, "202312110003", migrateAddExtraIndexes),
		construct(ctx, "202405301140", migrateAddReplayGainFields),
	}

	return gormigrate.
		New(db.DB, options, migrations).
		Migrate()
}

func construct(ctx MigrationContext, id string, f func(*gorm.DB, MigrationContext) error) *gormigrate.Migration {
	return constructNoTx(ctx, id, func(db *gorm.DB, ctx MigrationContext) error {
		return db.Transaction(func(tx *gorm.DB) error {
			return f(tx, ctx)
		})
	})
}

func constructNoTx(ctx MigrationContext, id string, f func(*gorm.DB, MigrationContext) error) *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: id,
		Migrate: func(db *gorm.DB) error {
			if err := f(db, ctx); err != nil {
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
	return tx.AutoMigrate(__OldPlaylist{}).Error
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
			tx.Where("id=?", id.Value).Preload("Podcast").Find(&pe)
			if pe.Filename == "" {
				return ""
			}
			return filepath.Join(ctx.PodcastsPath, fileutil.Safe(pe.Podcast.Title), pe.Filename)
		}
		return ""
	}

	store, err := playlist.NewStore(ctx.PlaylistsPath)
	if err != nil {
		return fmt.Errorf("create playlists store: %w", err)
	}

	var prevs []*__OldPlaylist
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

func migratePlayCountToLength(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(
		Play{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	// As a best guess, we set length played so far as length of album * current count / number of tracks in album
	step = tx.Exec(`
		UPDATE plays SET length=
		((SELECT SUM(length) FROM tracks WHERE tracks.album_id=plays.album_id)*plays.count/
		 (SELECT COUNT(*) FROM tracks WHERE tracks.album_id=plays.album_id));
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("calculate length: %w", err)
	}

	return nil
}

func migrateAlbumArtistsMany2Many(tx *gorm.DB, _ MigrationContext) error {
	// gorms seems to want to create the table automatically without ON DELETE rules
	step := tx.DropTableIfExists(AlbumArtist{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop prev: %w", err)
	}

	step = tx.AutoMigrate(
		AlbumArtist{},
		Album{},
		Artist{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	if tx.Dialect().HasColumn("albums", "tag_artist_id") {
		tx = tx.LogMode(false)
		step = tx.Exec(`
			INSERT INTO album_artists (album_id, artist_id)
			SELECT id album_id, tag_artist_id artist_id
			FROM albums
			WHERE tag_artist_id IS NOT NULL;
		`)
		if err := step.Error; err != nil && !strings.Contains(err.Error(), "no such column") {
			return fmt.Errorf("step insert from albums: %w", err)
		}

		step = tx.Exec(`DROP INDEX idx_albums_tag_artist_id`)
		if err := step.Error; err != nil {
			return fmt.Errorf("step drop index: %w", err)
		}

		step = tx.Exec(`ALTER TABLE albums DROP COLUMN tag_artist_id;`)
		if err := step.Error; err != nil {
			return fmt.Errorf("step drop albums tag artist id: %w", err)
		}
	}

	if tx.Dialect().HasColumn("tracks", "artist_id") {
		step = tx.Exec(`ALTER TABLE tracks DROP COLUMN artist_id;`)
		if err := step.Error; err != nil {
			return fmt.Errorf("step drop track tag artist: %w", err)
		}
	}

	return nil
}

func migrateDeleteArtistCoverField(tx *gorm.DB, _ MigrationContext) error {
	if !tx.Dialect().HasColumn("artists", "cover") {
		return nil
	}

	step := tx.Exec(`
		ALTER TABLE artists DROP COLUMN cover;
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop: %w", err)
	}

	return nil
}

func migrateArtistInfo(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		ArtistInfo{},
	).
		Error
}

func migratePlaylistsPaths(tx *gorm.DB, ctx MigrationContext) error {
	if !tx.Dialect().HasColumn("podcast_episodes", "path") {
		return nil
	}
	if !tx.Dialect().HasColumn("podcasts", "image_path") {
		return nil
	}

	step := tx.Exec(`
			ALTER TABLE podcasts RENAME COLUMN image_path TO image
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop podcast_episodes path: %w", err)
	}

	step = tx.AutoMigrate(
		Podcast{},
		PodcastEpisode{},
	)
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	var podcasts []*Podcast
	if err := tx.Find(&podcasts).Error; err != nil {
		return fmt.Errorf("step load: %w", err)
	}

	for _, p := range podcasts {
		p.Image = filepath.Base(p.Image)
		if err := tx.Save(p).Error; err != nil {
			return fmt.Errorf("saving podcast for cover %d: %w", p.ID, err)
		}

		oldPath, err := fileutil.First(
			filepath.Join(ctx.PodcastsPath, fileutil.Safe(p.Title)),
			filepath.Join(ctx.PodcastsPath, strings.ReplaceAll(p.Title, string(filepath.Separator), "_")), // old safe func
			filepath.Join(ctx.PodcastsPath, p.Title),
		)
		if err != nil {
			log.Printf("error finding old podcast path: %v. ignoring", err)
			continue
		}
		newPath := filepath.Join(ctx.PodcastsPath, fileutil.Safe(p.Title))
		p.RootDir = newPath
		if err := tx.Save(p).Error; err != nil {
			return fmt.Errorf("saving podcast %d: %w", p.ID, err)
		}
		if oldPath == newPath {
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("rename podcast path: %w", err)
		}
	}

	var podcastEpisodes []*PodcastEpisode
	if err := tx.Preload("Podcast").Find(&podcastEpisodes, "status=? OR status=?", PodcastEpisodeStatusCompleted, PodcastEpisodeStatusDownloading).Error; err != nil {
		return fmt.Errorf("step load: %w", err)
	}
	for _, pe := range podcastEpisodes {
		if pe.Filename == "" {
			continue
		}
		oldPath, err := fileutil.First(
			filepath.Join(pe.Podcast.RootDir, fileutil.Safe(pe.Filename)),
			filepath.Join(pe.Podcast.RootDir, strings.ReplaceAll(pe.Filename, string(filepath.Separator), "_")), // old safe func
			filepath.Join(pe.Podcast.RootDir, pe.Filename),
		)
		if err != nil {
			log.Printf("error finding old podcast episode path: %v. ignoring", err)
			continue
		}
		newName := fileutil.Safe(filepath.Base(oldPath))
		pe.Filename = newName
		if err := tx.Save(pe).Error; err != nil {
			return fmt.Errorf("saving podcast episode %d: %w", pe.ID, err)
		}
		newPath := filepath.Join(pe.Podcast.RootDir, newName)
		if oldPath == newPath {
			continue
		}
		if err := os.Rename(oldPath, newPath); err != nil {
			return fmt.Errorf("rename podcast episode path: %w", err)
		}
	}

	step = tx.Exec(`
			ALTER TABLE podcast_episodes DROP COLUMN path
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop podcast_episodes path: %w", err)
	}
	return nil
}

func backupDBPre016(tx *gorm.DB, ctx MigrationContext) error {
	if !ctx.Production {
		return nil
	}
	backupPath := fmt.Sprintf("%s.%d.bak", ctx.DBPath, time.Now().Unix())
	sandbox.ReadWriteCreatePath(backupPath)
	return Dump(context.Background(), tx, backupPath)
}

func migrateAlbumTagArtistString(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Album{}).Error
}

func migrateTrackArtists(tx *gorm.DB, _ MigrationContext) error {
	// gorms seems to want to create the table automatically without ON DELETE rules
	step := tx.DropTableIfExists(TrackArtist{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop prev: %w", err)
	}
	return tx.AutoMigrate(TrackArtist{}).Error
}

func migrateArtistAppearances(tx *gorm.DB, _ MigrationContext) error {
	// gorms seems to want to create the table automatically without ON DELETE rules
	step := tx.DropTableIfExists(ArtistAppearances{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop prev: %w", err)
	}

	step = tx.AutoMigrate(ArtistAppearances{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	step = tx.Exec(`
		INSERT INTO artist_appearances (artist_id, album_id)
		SELECT artist_id, album_id
		FROM album_artists
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step transfer album artists: %w", err)
	}

	step = tx.Exec(`
		INSERT OR IGNORE INTO artist_appearances (artist_id, album_id)
		SELECT track_artists.artist_id, tracks.album_id
		FROM track_artists
		JOIN tracks ON tracks.id=track_artists.track_id
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step transfer album artists: %w", err)
	}

	return nil
}

func migrateAlbumInfo(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		AlbumInfo{},
	).
		Error
}

func migrateTemporaryDisplayAlbumArtist(tx *gorm.DB, _ MigrationContext) error {
	// keep some things working so that people have an album.tag_artist_id until their next full scan
	return tx.Exec(`
		UPDATE albums
		SET tag_album_artist=(
			SELECT group_concat(artists.name, ', ')
			FROM artists
			JOIN album_artists ON album_artists.artist_id=artists.id AND album_artists.album_id=albums.id
			GROUP BY album_artists.album_id
		)
		WHERE tag_album_artist=''
	`).Error
}

func migrateAddExtraIndexes(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`
		CREATE INDEX idx_track_genres_genre_id ON "track_genres" (genre_id);
		CREATE INDEX idx_album_genres_genre_id ON "album_genres" (genre_id);
		CREATE INDEX idx_album_artists_artist_id ON "album_artists" (artist_id);
		CREATE INDEX idx_track_artists_artist_id ON "track_artists" (artist_id);
		CREATE INDEX idx_artist_appearances_album_id ON "artist_appearances" (album_id);
	`).Error
}

func migrateAddReplayGainFields(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Track{}).Error
}
