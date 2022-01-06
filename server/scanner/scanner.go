package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/rainycape/unidecode"

	"go.senan.xyz/gonic/multierr"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/mime"
	"go.senan.xyz/gonic/server/scanner/tags"
)

var (
	ErrAlreadyScanning = errors.New("already scanning")
	ErrReadingTags     = errors.New("could not read tags")
)

type Scanner struct {
	db         *db.DB
	musicDirs  []string
	genreSplit string
	tagger     tags.Reader
	scanning   *int32
}

func New(musicDirs []string, db *db.DB, genreSplit string, tagger tags.Reader) *Scanner {
	return &Scanner{
		db:         db,
		musicDirs:  musicDirs,
		genreSplit: genreSplit,
		tagger:     tagger,
		scanning:   new(int32),
	}
}

func (s *Scanner) IsScanning() bool {
	return atomic.LoadInt32(s.scanning) == 1
}

type ScanOptions struct {
	IsFull bool
}

func (s *Scanner) ScanAndClean(opts ScanOptions) error {
	if s.IsScanning() {
		return ErrAlreadyScanning
	}
	atomic.StoreInt32(s.scanning, 1)
	defer atomic.StoreInt32(s.scanning, 0)

	start := time.Now()
	c := &ctx{
		full:       opts.IsFull,
		errs:       &multierr.Err{},
		seenTracks: map[int]struct{}{},
		seenAlbums: map[int]struct{}{},
	}

	log.Println("starting scan")
	defer func() {
		log.Printf("finished scan in %s, +%d/%d tracks (%d err)\n",
			durSince(start), c.seenTracksNew, len(c.seenTracks), c.errs.Len())
	}()

	for _, dir := range s.musicDirs {
		err := filepath.WalkDir(dir, func(absPath string, d fs.DirEntry, err error) error {
			return s.scanCallback(c, dir, absPath, d, err)
		})
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}
	}

	if err := s.cleanTracks(c.seenTracks); err != nil {
		return fmt.Errorf("clean tracks: %w", err)
	}
	if err := s.cleanAlbums(c.seenAlbums); err != nil {
		return fmt.Errorf("clean albums: %w", err)
	}
	if err := s.cleanArtists(); err != nil {
		return fmt.Errorf("clean artists: %w", err)
	}
	if err := s.cleanGenres(); err != nil {
		return fmt.Errorf("clean genres: %w", err)
	}

	if err := s.db.SetSetting("last_scan_time", strconv.FormatInt(time.Now().Unix(), 10)); err != nil {
		return fmt.Errorf("set scan time: %w", err)
	}

	if c.errs.Len() > 0 {
		return c.errs
	}

	return nil
}

func (s *Scanner) scanCallback(c *ctx, dir string, absPath string, d fs.DirEntry, err error) error {
	if err != nil {
		c.errs.Add(err)
		return nil
	}
	if dir == absPath {
		return nil
	}

	switch d.Type() {
	case os.ModeDir:
	case os.ModeSymlink:
		eval, _ := filepath.EvalSymlinks(absPath)
		return filepath.WalkDir(eval, func(subAbs string, d fs.DirEntry, err error) error {
			subAbs = strings.Replace(subAbs, eval, absPath, 1)
			return s.scanCallback(c, dir, subAbs, d, err)
		})
	default:
		return nil
	}

	log.Printf("processing folder `%s`", absPath)

	tx := s.db.Begin()
	if err := s.scanDir(tx, c, dir, absPath); err != nil {
		c.errs.Add(fmt.Errorf("%q: %w", absPath, err))
		tx.Rollback()
		return nil
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func (s *Scanner) scanDir(tx *db.DB, c *ctx, musicDir string, absPath string) error {
	items, err := os.ReadDir(absPath)
	if err != nil {
		return err
	}

	var tracks []string
	var cover string
	for _, item := range items {
		if isCover(item.Name()) {
			cover = item.Name()
			continue
		}
		if _, ok := mime.FromExtension(ext(item.Name())); ok {
			tracks = append(tracks, item.Name())
			continue
		}
	}

	relPath, _ := filepath.Rel(musicDir, absPath)
	pdir, pbasename := filepath.Split(filepath.Dir(relPath))
	parent := &db.Album{}
	if err := tx.Where(db.Album{RootDir: musicDir, LeftPath: pdir, RightPath: pbasename}).FirstOrCreate(parent).Error; err != nil {
		return fmt.Errorf("first or create parent: %w", err)
	}

	c.seenAlbums[parent.ID] = struct{}{}

	dir, basename := filepath.Split(relPath)
	album := &db.Album{}
	if err := tx.Where(db.Album{RootDir: musicDir, LeftPath: dir, RightPath: basename}).First(album).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find album: %w", err)
	}

	if err := populateAlbumBasics(tx, musicDir, parent, album, dir, basename, cover); err != nil {
		return fmt.Errorf("populate album basics: %w", err)
	}

	c.seenAlbums[album.ID] = struct{}{}

	sort.Strings(tracks)
	for i, basename := range tracks {
		absPath := filepath.Join(musicDir, relPath, basename)
		if err := s.populateTrackAndAlbumArtists(tx, c, i, album, basename, absPath); err != nil {
			return fmt.Errorf("populate track %q: %w", basename, err)
		}
	}

	return nil
}

func (s *Scanner) populateTrackAndAlbumArtists(tx *db.DB, c *ctx, i int, album *db.Album, basename string, absPath string) error {
	track := &db.Track{AlbumID: album.ID, Filename: filepath.Base(basename)}
	if err := tx.Where(track).First(track).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("query track: %w", err)
	}

	c.seenTracks[track.ID] = struct{}{}

	stat, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stating %q: %w", basename, err)
	}
	if !c.full && stat.ModTime().Before(track.UpdatedAt) {
		return nil
	}

	trags, err := s.tagger.Read(absPath)
	if err != nil {
		return fmt.Errorf("%v: %w", err, ErrReadingTags)
	}

	artistName := trags.SomeAlbumArtist()
	albumArtist, err := s.populateAlbumArtist(tx, album, artistName)
	if err != nil {
		return fmt.Errorf("populate artist: %w", err)
	}

	if err := populateTrack(tx, album, albumArtist, track, trags, basename, int(stat.Size())); err != nil {
		return fmt.Errorf("process %q: %w", basename, err)
	}

	c.seenTracks[track.ID] = struct{}{}
	c.seenTracksNew++

	genreNames := strings.Split(trags.SomeGenre(), s.genreSplit)
	genreIDs, err := s.populateGenres(tx, track, genreNames)
	if err != nil {
		return fmt.Errorf("populate genres: %w", err)
	}

	if err := s.populateTrackGenres(tx, track, genreIDs); err != nil {
		return fmt.Errorf("propulate track genres: %w", err)
	}

	// metadata for the album table comes only from the the first track's tags
	if i > 0 {
		return nil
	}

	stat_t := stat.Sys().(*syscall.Stat_t)

	if err := populateAlbum(tx, album, albumArtist, trags, stat.ModTime(), timespecToTime(stat_t.Ctim)); err != nil {
		return fmt.Errorf("populate album: %w", err)
	}

	if err := populateAlbumGenres(tx, album, genreIDs); err != nil {
		return fmt.Errorf("populate album genres: %w", err)
	}

	return nil
}

func populateAlbum(tx *db.DB, album *db.Album, albumArtist *db.Artist, trags tags.Parser, modTime time.Time, createTime time.Time) error {
	albumName := trags.SomeAlbum()
	album.TagTitle = albumName
	album.TagTitleUDec = decoded(albumName)
	album.TagBrainzID = trags.AlbumBrainzID()
	album.TagYear = trags.Year()
	album.TagArtistID = albumArtist.ID
	album.ModifiedAt = modTime
	album.CreatedAt = createTime

	if err := tx.Save(&album).Error; err != nil {
		return fmt.Errorf("saving album: %w", err)
	}

	return nil
}

func timespecToTime(ts syscall.Timespec) time.Time {
	return time.Unix(int64(ts.Sec), int64(ts.Nsec))
}

func populateAlbumBasics(tx *db.DB, rootAbsPath string, parent, album *db.Album, dir, basename string, cover string) error {
	album.RootDir = rootAbsPath
	album.LeftPath = dir
	album.RightPath = basename
	album.Cover = cover
	album.RightPathUDec = decoded(basename)
	album.ParentID = parent.ID

	if err := tx.Save(&album).Error; err != nil {
		return fmt.Errorf("saving album: %w", err)
	}

	return nil
}

func populateTrack(tx *db.DB, album *db.Album, albumArtist *db.Artist, track *db.Track, trags tags.Parser, absPath string, size int) error {
	basename := filepath.Base(absPath)
	track.Filename = basename
	track.FilenameUDec = decoded(basename)
	track.Size = size
	track.AlbumID = album.ID
	track.ArtistID = albumArtist.ID

	track.TagTitle = trags.Title()
	track.TagTitleUDec = decoded(trags.Title())
	track.TagTrackArtist = trags.Artist()
	track.TagTrackNumber = trags.TrackNumber()
	track.TagDiscNumber = trags.DiscNumber()
	track.TagBrainzID = trags.BrainzID()

	track.Length = trags.Length()   // these two should be calculated
	track.Bitrate = trags.Bitrate() // ...from the file instead of tags

	if err := tx.Save(&track).Error; err != nil {
		return fmt.Errorf("saving track: %w", err)
	}

	return nil
}

func (s *Scanner) populateAlbumArtist(tx *db.DB, guessedArtistFolder *db.Album, artistName string) (*db.Artist, error) {
	var artist db.Artist
	update := db.Artist{
		Name:     artistName,
		NameUDec: decoded(artistName),
	}
	if guessedArtistFolder != nil {
		update.GuessedFolderID = guessedArtistFolder.ParentID
	}
	if err := tx.Where("name=?", artistName).Assign(update).FirstOrCreate(&artist).Error; err != nil {
		return nil, fmt.Errorf("find or create artist: %w", err)
	}
	return &artist, nil
}

func (s *Scanner) populateGenres(tx *db.DB, track *db.Track, names []string) ([]int, error) {
	var filteredNames []string
	for _, name := range names {
		if clean := strings.TrimSpace(name); clean != "" {
			filteredNames = append(filteredNames, clean)
		}
	}
	if len(filteredNames) == 0 {
		return []int{}, nil
	}
	var ids []int
	for _, name := range filteredNames {
		var genre db.Genre
		if err := tx.FirstOrCreate(&genre, db.Genre{Name: name}).Error; err != nil {
			return nil, fmt.Errorf("find or create genre: %w", err)
		}
		ids = append(ids, genre.ID)
	}
	return ids, nil
}

func (s *Scanner) populateTrackGenres(tx *db.DB, track *db.Track, genreIDs []int) error {
	if err := tx.Where("track_id=?", track.ID).Delete(db.TrackGenre{}).Error; err != nil {
		return fmt.Errorf("delete old track genre records: %w", err)
	}

	if err := tx.InsertBulkLeftMany("track_genres", []string{"track_id", "genre_id"}, track.ID, genreIDs); err != nil {
		return fmt.Errorf("insert bulk track genres: %w", err)
	}
	return nil
}

func populateAlbumGenres(tx *db.DB, album *db.Album, genreIDs []int) error {
	if err := tx.Where("album_id=?", album.ID).Delete(db.AlbumGenre{}).Error; err != nil {
		return fmt.Errorf("delete old album genre records: %w", err)
	}

	if err := tx.InsertBulkLeftMany("album_genres", []string{"album_id", "genre_id"}, album.ID, genreIDs); err != nil {
		return fmt.Errorf("insert bulk album genres: %w", err)
	}
	return nil
}

func (s *Scanner) cleanTracks(seenTracks map[int]struct{}) error {
	start := time.Now()
	var previous []int
	var missing []int64
	err := s.db.
		Model(&db.Track{}).
		Pluck("id", &previous).
		Error
	if err != nil {
		return fmt.Errorf("plucking ids: %w", err)
	}
	for _, prev := range previous {
		if _, ok := seenTracks[prev]; !ok {
			missing = append(missing, int64(prev))
		}
	}
	err = s.db.TransactionChunked(missing, func(tx *gorm.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Track{}).Error
	})
	if err != nil {
		return err
	}
	log.Printf("finished clean tracks in %s, %d removed", durSince(start), len(missing))
	return nil
}

func (s *Scanner) cleanAlbums(seenAlbums map[int]struct{}) error {
	start := time.Now()
	var previous []int
	var missing []int64
	err := s.db.
		Model(&db.Album{}).
		Pluck("id", &previous).
		Error
	if err != nil {
		return fmt.Errorf("plucking ids: %w", err)
	}
	for _, prev := range previous {
		if _, ok := seenAlbums[prev]; !ok {
			missing = append(missing, int64(prev))
		}
	}
	err = s.db.TransactionChunked(missing, func(tx *gorm.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Album{}).Error
	})
	if err != nil {
		return err
	}
	log.Printf("finished clean albums in %s, %d removed", durSince(start), len(missing))
	return nil
}

func (s *Scanner) cleanArtists() error {
	start := time.Now()
	sub := s.db.
		Select("artists.id").
		Model(&db.Artist{}).
		Joins("LEFT JOIN albums ON albums.tag_artist_id=artists.id").
		Where("albums.id IS NULL").
		SubQuery()
	q := s.db.
		Where("artists.id IN ?", sub).
		Delete(&db.Artist{})
	if err := q.Error; err != nil {
		return err
	}
	log.Printf("finished clean artists in %s, %d removed", durSince(start), q.RowsAffected)
	return nil
}

func (s *Scanner) cleanGenres() error {
	start := time.Now()
	subTrack := s.db.
		Select("genres.id").
		Model(&db.Genre{}).
		Joins("LEFT JOIN track_genres ON track_genres.genre_id=genres.id").
		Where("track_genres.genre_id IS NULL").
		SubQuery()
	subAlbum := s.db.
		Select("genres.id").
		Model(&db.Genre{}).
		Joins("LEFT JOIN album_genres ON album_genres.genre_id=genres.id").
		Where("album_genres.genre_id IS NULL").
		SubQuery()
	q := s.db.
		Where("genres.id IN ? AND genres.id IN ?", subTrack, subAlbum).
		Delete(&db.Genre{})
	log.Printf("finished clean genres in %s, %d removed", durSince(start), q.RowsAffected)
	return nil
}

func ext(name string) string {
	ext := filepath.Ext(name)
	if len(ext) == 0 {
		return ""
	}
	return ext[1:]
}

func isCover(name string) bool {
	switch path := strings.ToLower(name); path {
	case
		"cover.png", "cover.jpg", "cover.jpeg",
		"folder.png", "folder.jpg", "folder.jpeg",
		"album.png", "album.jpg", "album.jpeg",
		"albumart.png", "albumart.jpg", "albumart.jpeg",
		"front.png", "front.jpg", "front.jpeg",
		"artist.png", "artist.jpg", "artist.jpeg":
		return true
	default:
		return false
	}
}

// decoded converts a string to it's latin equivalent.
// it will be used by the model's *UDec fields, and is only set if it
// differs from the original. the fields are used for searching.
func decoded(in string) string {
	if u := unidecode.Unidecode(in); u != in {
		return u
	}
	return ""
}

func durSince(t time.Time) time.Duration {
	return time.Since(t).Truncate(10 * time.Microsecond)
}

type ctx struct {
	errs          *multierr.Err
	full          bool
	seenTracks    map[int]struct{}
	seenAlbums    map[int]struct{}
	seenTracksNew int
}
