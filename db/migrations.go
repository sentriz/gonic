//nolint:err113,goconst
package db

import (
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
		construct(ctx, "202501152035", migrateTrackAddIndexOnAlbumID),
		construct(ctx, "202501152036", migrateAlbumAddIndexOnParentID),
		construct(ctx, "202502012036", migratePodcastEpisode),
		construct(ctx, "202504132036", migratePodcast),
		construct(ctx, "202505211202", migrateTrackAddIndexOnBrainzID),
		construct(ctx, "202505262025", migrateAlbumAddIndexOnBrainzID),
		construct(ctx, "202507062103", migrateAlbumCompilationReleaseType),
		construct(ctx, "202508261102", migrateAddLyrics),
		construct(ctx, "202509291741", migrateAddAlbumEmbeddedCoverTrackID),
		construct(ctx, "202509301448", migrateAddTrackEmbeddedCover),
		construct(ctx, "202512021147", migrateAlbumAddIndexOnCreatedAt),
		construct(ctx, "202601201000", migrateAddAlbumDiscTitles),
		construct(ctx, "202602061800", migrateAddTrackYear),
		construct(ctx, "202604231200", migrateAddTrackContributors),
		construct(ctx, "202604280000", migrateAddCreditedAs),
		construct(ctx, "202604281200", migrateUnifyCredits),
		construct(ctx, "202605061812", migrateAddGenreInherited),
		construct(ctx, "202605081200", migrateAddTrackIsrc),
		construct(ctx, "202605131200", migrateAddArtistCreditDisplay),
		construct(ctx, "202605221200", migrateAddAlbumLabel),
		construct(ctx, "202605222200", migrateAddTrackPlays),
		construct(ctx, "202605231200", migrateAddTranscodeFormatPreference),
		construct(ctx, "202605251200", migrateDeleteUnknownGenre),
		construct(ctx, "202605261200", migrateDeleteLastfmPlaceholderArtistImage),
		construct(ctx, "202606041200", migrateArtistMusicBrainzID),
		construct(ctx, "202606051200", migrateDropAverageRating),
		construct(ctx, "202606091200", migrateArtistInfoFieldsBySource),
		construct(ctx, "202606091300", migrateAlbumInfoMusicBrainzDisambiguation),
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
		__OldPlay{},
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
		__OldPlay{},
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
	step := tx.DropTableIfExists(__OldAlbumArtist{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop prev: %w", err)
	}

	step = tx.AutoMigrate(
		__OldAlbumArtist{},
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

func migrateAlbumTagArtistString(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Album{}).Error
}

func migrateTrackArtists(tx *gorm.DB, _ MigrationContext) error {
	// gorms seems to want to create the table automatically without ON DELETE rules
	step := tx.DropTableIfExists(__OldTrackArtist{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop prev: %w", err)
	}
	return tx.AutoMigrate(__OldTrackArtist{}).Error
}

func migrateArtistAppearances(tx *gorm.DB, _ MigrationContext) error {
	// gorms seems to want to create the table automatically without ON DELETE rules
	step := tx.DropTableIfExists(__OldArtistAppearances{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step drop prev: %w", err)
	}

	step = tx.AutoMigrate(__OldArtistAppearances{})
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

func migrateTrackAddIndexOnAlbumID(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`
		CREATE INDEX idx_tracks_album_id ON "tracks" (album_id, length);
	`).Error
}

func migrateAlbumAddIndexOnParentID(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`
		CREATE INDEX idx_albums_parent_id ON "albums" (parent_id);
	`).Error
}

func migratePodcastEpisode(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		PodcastEpisode{},
	).Error
}

func migrateTrackAddIndexOnBrainzID(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`
		CREATE INDEX idx_tracks_brainz_id ON "tracks" (tag_brainz_id);
		`).Error
}

func migrateAlbumAddIndexOnBrainzID(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`
		CREATE INDEX idx_albums_brainz_id ON "albums" (tag_brainz_id);
		`).Error
}

func migrateAlbumCompilationReleaseType(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Album{},
	).Error
}

func migrateAddLyrics(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Track{}).Error
}

func migrateAddAlbumEmbeddedCoverTrackID(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Album{}).Error
}

func migrateAddTrackEmbeddedCover(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Track{}).Error
}

func migrateAlbumAddIndexOnCreatedAt(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`
		CREATE INDEX idx_albums_created_at ON "albums" (created_at);
		`).Error
}

func migrateAddAlbumDiscTitles(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(AlbumDiscTitle{}).Error
}

func migrateAddTrackContributors(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(__OldTrackContributor{}).Error
}

func migrateAddCreditedAs(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(__OldTrackArtist{}, __OldAlbumArtist{}, __OldTrackContributor{}).Error
}

func migrateUnifyCredits(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(AlbumCredit{}, TrackCredit{})
	if err := step.Error; err != nil {
		return fmt.Errorf("auto migrate credit tables: %w", err)
	}

	backfills := []string{
		`INSERT INTO album_credits (album_id, artist_id, role, credited_as)
			SELECT album_id, artist_id, 'albumartist', credited_as FROM album_artists`,
		`INSERT INTO track_credits (track_id, artist_id, role, credited_as)
			SELECT track_id, artist_id, 'artist', credited_as FROM track_artists`,
		`INSERT INTO track_credits (track_id, artist_id, role, credited_as)
			SELECT track_id, artist_id, role, credited_as FROM track_contributors`,
	}
	for _, q := range backfills {
		if err := tx.Exec(q).Error; err != nil {
			return fmt.Errorf("backfill: %w", err)
		}
	}

	for _, t := range []string{"album_artists", "track_artists", "track_contributors", "artist_appearances"} {
		if err := tx.Exec(`DROP TABLE ` + t).Error; err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	return nil
}

func migrateAddTrackYear(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(Track{})
	if err := step.Error; err != nil {
		return fmt.Errorf("step auto migrate: %w", err)
	}

	step = tx.Exec(`
	UPDATE tracks
	SET tag_year=(SELECT tag_year
	FROM albums
	WHERE tracks.album_id = albums.id)
	`)
	if err := step.Error; err != nil {
		return fmt.Errorf("step set default track year based on album's : %w", err)
	}

	return nil
}

func migrateAddGenreInherited(tx *gorm.DB, _ MigrationContext) error {
	step := tx.AutoMigrate(TrackGenre{}, AlbumGenre{})
	return step.Error
}

func migrateAddTrackIsrc(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(
		Track{},
		TrackISRC{},
	).Error
}

func migrateAddArtistCreditDisplay(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(Album{}, Track{}).Error
}

func migrateAddAlbumLabel(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(AlbumLabel{}).Error
}

func migrateAddTrackPlays(tx *gorm.DB, _ MigrationContext) error {
	if err := tx.AutoMigrate(TrackPlay{}).Error; err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// distribute each album's play count evenly across its tracks as a
	// fractional count, so SUM(track count) per album == old album count.
	// length is split the same way but kept as integer seconds. plays is
	// aggregated first since historic races could have left duplicate
	// (user, album) rows.
	if err := tx.Exec(`
		INSERT INTO track_plays (user_id, track_id, time, count, length)
		SELECT
			agg.user_id,
			tracks.id,
			agg.time,
			CAST(agg.count AS REAL) / track_counts.n,
			agg.length / track_counts.n
		FROM (
			SELECT user_id, album_id, MAX(time) AS time, SUM(count) AS count, SUM(length) AS length
			FROM plays
			GROUP BY user_id, album_id
		) agg
		JOIN tracks ON tracks.album_id = agg.album_id
		JOIN (
			SELECT album_id, COUNT(*) AS n FROM tracks GROUP BY album_id
		) track_counts ON track_counts.album_id = agg.album_id;
	`).Error; err != nil {
		return fmt.Errorf("backfill from plays: %w", err)
	}

	return tx.Exec(`DROP TABLE plays;`).Error
}

func migrateAddTranscodeFormatPreference(tx *gorm.DB, _ MigrationContext) error {
	return tx.AutoMigrate(TranscodeFormatPreference{}).Error
}

func migrateDeleteUnknownGenre(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`DELETE FROM genres WHERE name='Unknown Genre'`).Error
}

func migrateDeleteLastfmPlaceholderArtistImage(tx *gorm.DB, _ MigrationContext) error {
	return tx.Exec(`UPDATE artist_infos SET image_url='' WHERE image_url='https://lastfm.freetls.fastly.net/i/u/ar0/2a96cbd8b46e442fc41c2b86b821562f.jpg'`).Error
}

func migrateDropAverageRating(tx *gorm.DB, _ MigrationContext) error {
	for table, column := range map[string]string{
		"albums":  "average_rating",
		"artists": "average_rating",
		"tracks":  "average_rating",
	} {
		if !tx.Dialect().HasColumn(table, column) {
			continue
		}
		if err := tx.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)).Error; err != nil {
			return fmt.Errorf("drop %s.%s: %w", table, column, err)
		}
	}
	return nil
}

func migrateArtistMusicBrainzID(tx *gorm.DB, _ MigrationContext) error {
	if err := tx.AutoMigrate(Artist{}).Error; err != nil {
		return fmt.Errorf("automigrate artist: %w", err)
	}
	return tx.Exec(`
		DROP INDEX IF EXISTS uix_artists_name;
		CREATE INDEX IF NOT EXISTS idx_artists_name ON artists (name);
		CREATE UNIQUE INDEX IF NOT EXISTS uix_artists_music_brainz_id ON artists (music_brainz_id) WHERE music_brainz_id != '';
	`).Error
}

func migrateArtistInfoFieldsBySource(tx *gorm.DB, _ MigrationContext) error {
	// rename existing lastfm-sourced columns to be source-prefixed. must run before AutoMigrate,
	// else AutoMigrate creates the new names empty (from the current struct) and the rename collides.
	for _, r := range [][2]string{
		{"biography", "last_fm_biography"},
		{"top_tracks", "last_fm_top_tracks"},
		{"similar_artists", "last_fm_similar_artists"},
	} {
		if !tx.Dialect().HasColumn("artist_infos", r[0]) {
			continue
		}
		if err := tx.Exec(fmt.Sprintf("ALTER TABLE artist_infos RENAME COLUMN %q TO %q", r[0], r[1])).Error; err != nil {
			return fmt.Errorf("rename column %s: %w", r[0], err)
		}
	}
	return tx.AutoMigrate(ArtistInfo{}).Error
}

func migrateAlbumInfoMusicBrainzDisambiguation(tx *gorm.DB, _ MigrationContext) error {
	// rename the existing lastfm-sourced column to be source-prefixed. must run before AutoMigrate,
	// else AutoMigrate creates the new name empty (from the current struct) and the rename collides.
	if tx.Dialect().HasColumn("album_infos", "notes") {
		if err := tx.Exec(`ALTER TABLE album_infos RENAME COLUMN "notes" TO "last_fm_notes"`).Error; err != nil {
			return fmt.Errorf("rename column notes: %w", err)
		}
	}
	return tx.AutoMigrate(AlbumInfo{}).Error
}
