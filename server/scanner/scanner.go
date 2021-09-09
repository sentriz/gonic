package scanner

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
	"github.com/rainycape/unidecode"

	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/mime"
	"go.senan.xyz/gonic/server/scanner/stack"
	"go.senan.xyz/gonic/server/scanner/tags"
)

var (
	ErrAlreadyScanning = errors.New("already scanning")
	ErrStatingItem     = errors.New("could not stat item")
	ErrReadingTags     = errors.New("could not read tags")
)

func durSince(t time.Time) time.Duration {
	return time.Since(t).Truncate(10 * time.Microsecond)
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

// isScanning acts as an atomic boolean semaphore. we don't
// want to have more than one scan going on at a time
var isScanning int32 //nolint:gochecknoglobals

func IsScanning() bool {
	return atomic.LoadInt32(&isScanning) == 1
}

func SetScanning() func() {
	atomic.StoreInt32(&isScanning, 1)
	return func() {
		atomic.StoreInt32(&isScanning, 0)
	}
}

type Scanner struct {
	db         *db.DB
	musicPath  string
	isFull     bool
	genreSplit string
	// these two are for the transaction we do for every album.
	// the boolean is there so we dont begin or commit multiple
	// times in the handle album or post children callback
	trTx     *db.DB
	trTxOpen bool
	// these two are for keeping state between noted in the tree.
	// eg. keep track of a parents album or the path to a cover
	// we just saw that we need to commit in the post children
	// callback
	curAlbums *stack.Stack
	curCover  string
	// then the rest are for stats and cleanup at the very end
	seenTracks    map[int]struct{} // set of p keys
	seenAlbums    map[int]struct{} // set of p keys
	seenTracksNew int              // n tracks not seen before
}

func New(musicPath string, db *db.DB, genreSplit string) *Scanner {
	return &Scanner{
		db:         db,
		musicPath:  musicPath,
		genreSplit: genreSplit,
	}
}

// ## begin clean funcs
// ## begin clean funcs
// ## begin clean funcs

func (s *Scanner) cleanTracks() error {
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
		if _, ok := s.seenTracks[prev]; !ok {
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

func (s *Scanner) cleanAlbums() error {
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
		if _, ok := s.seenAlbums[prev]; !ok {
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
		Where("genres.id IN ?", subTrack).
		Or("genres.id IN ?", subAlbum).
		Delete(&db.Genre{})
	log.Printf("finished clean genres in %s, %d removed", durSince(start), q.RowsAffected)
	return nil
}

// ## begin entries
// ## begin entries
// ## begin entries

type ScanOptions struct {
	IsFull bool
	// TODO https://github.com/sentriz/gonic/issues/64
	Path string
}

func (s *Scanner) Start(opts ScanOptions) error {
	if IsScanning() {
		return ErrAlreadyScanning
	}
	unSet := SetScanning()
	defer unSet()

	// reset state vars for the new scan
	s.isFull = opts.IsFull
	s.seenTracks = map[int]struct{}{}
	s.seenAlbums = map[int]struct{}{}
	s.curAlbums = &stack.Stack{}
	s.seenTracksNew = 0

	// begin walking
	log.Println("starting scan")
	var errCount int
	start := time.Now()
	err := godirwalk.Walk(s.musicPath, &godirwalk.Options{
		Callback:             s.callbackItem,
		PostChildrenCallback: s.callbackPost,
		Unsorted:             true,
		FollowSymbolicLinks:  true,
		ErrorCallback: func(path string, err error) godirwalk.ErrorAction {
			log.Printf("error processing `%s`: %v", path, err)
			errCount++
			return godirwalk.SkipNode
		},
	})
	if err != nil {
		return fmt.Errorf("walking filesystem: %w", err)
	}
	log.Printf("finished scan in %s, +%d/%d tracks (%d err)\n",
		durSince(start),
		s.seenTracksNew,
		len(s.seenTracks),
		errCount,
	)

	if err := s.cleanTracks(); err != nil {
		return fmt.Errorf("clean tracks: %w", err)
	}
	if err := s.cleanAlbums(); err != nil {
		return fmt.Errorf("clean albums: %w", err)
	}
	if err := s.cleanArtists(); err != nil {
		return fmt.Errorf("clean artists: %w", err)
	}
	if err := s.cleanGenres(); err != nil {
		return fmt.Errorf("clean genres: %w", err)
	}

	// finish up
	strNow := strconv.FormatInt(time.Now().Unix(), 10)
	s.db.SetSetting("last_scan_time", strNow)
	return nil
}

// items are passed to the handle*() functions
type item struct {
	fullPath  string
	relPath   string
	directory string
	filename  string
	stat      os.FileInfo
}

func isCover(filename string) bool {
	filename = strings.ToLower(filename)
	known := map[string]struct{}{
		"cover.png":     {},
		"cover.jpg":     {},
		"cover.jpeg":    {},
		"folder.png":    {},
		"folder.jpg":    {},
		"folder.jpeg":   {},
		"album.png":     {},
		"album.jpg":     {},
		"album.jpeg":    {},
		"albumart.png":  {},
		"albumart.jpg":  {},
		"albumart.jpeg": {},
		"front.png":     {},
		"front.jpg":     {},
		"front.jpeg":    {},
	}
	_, ok := known[filename]
	return ok
}

// ## begin callbacks
// ## begin callbacks
// ## begin callbacks

func (s *Scanner) callbackItem(fullPath string, info *godirwalk.Dirent) error {
	stat, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrStatingItem, err)
	}
	relPath, err := filepath.Rel(s.musicPath, fullPath)
	if err != nil {
		return fmt.Errorf("getting relative path: %w", err)
	}
	directory, filename := path.Split(relPath)
	it := &item{
		fullPath:  fullPath,
		relPath:   relPath,
		directory: directory,
		filename:  filename,
		stat:      stat,
	}
	isDir, err := info.IsDirOrSymlinkToDir()
	if err != nil {
		return fmt.Errorf("stating link to dir: %w", err)
	}
	if isDir {
		return s.handleAlbum(it)
	}
	if isCover(filename) {
		s.curCover = filename
		return nil
	}
	ext := path.Ext(filename)
	if ext == "" {
		return nil
	}
	if _, ok := mime.FromExtension(ext[1:]); ok {
		return s.handleTrack(it)
	}
	return nil
}

func (s *Scanner) callbackPost(fullPath string, info *godirwalk.Dirent) error {
	defer func() {
		s.curCover = ""
	}()
	if s.trTxOpen {
		s.trTx.Commit()
		s.trTxOpen = false
	}
	// begin taking the current album off the stack and add it's
	// parent, cover that we found, etc.
	album := s.curAlbums.Pop()
	if album.ParentID != 0 {
		return nil
	}
	album.ParentID = s.curAlbums.PeekID()
	album.Cover = s.curCover
	if err := s.db.Save(album).Error; err != nil {
		return fmt.Errorf("writing albums table: %w", err)
	}
	// we only log changed albums
	log.Printf("processed folder `%s`\n",
		path.Join(album.LeftPath, album.RightPath))
	return nil
}

// ## begin handlers
// ## begin handlers
// ## begin handlers

func (s *Scanner) itemUnchanged(statModTime, updatedInDB time.Time) bool {
	if s.isFull {
		return false
	}
	return statModTime.Before(updatedInDB)
}

func (s *Scanner) handleAlbum(it *item) error {
	if s.trTxOpen {
		// a transaction still being open when we handle an album can
		// happen if there is a album that contains /both/ tracks and
		// sub albums
		s.trTx.Commit()
		s.trTxOpen = false
	}
	album := &db.Album{}
	defer func() {
		// album's id will come from early return
		// or save at the end
		s.seenAlbums[album.ID] = struct{}{}
		s.curAlbums.Push(album)
	}()
	err := s.db.
		Where(db.Album{
			LeftPath:  it.directory,
			RightPath: it.filename,
		}).
		First(album).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		s.itemUnchanged(it.stat.ModTime(), album.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	album.LeftPath = it.directory
	album.RightPath = it.filename
	album.RightPathUDec = decoded(it.filename)
	album.ModifiedAt = it.stat.ModTime()
	if err := s.db.Save(album).Error; err != nil {
		return fmt.Errorf("writing albums table: %w", err)
	}
	return nil
}

func (s *Scanner) handleTrack(it *item) error {
	if !s.trTxOpen {
		s.trTx = s.db.Begin()
		s.trTxOpen = true
	}

	// init empty track and mark its ID (from lookup or save)
	// for later cleanup later
	var track db.Track
	defer func() {
		s.seenTracks[track.ID] = struct{}{}
	}()

	album := s.curAlbums.Peek()
	err := s.trTx.
		Select("id, updated_at").
		Where(db.Track{
			AlbumID:  album.ID,
			Filename: it.filename,
		}).
		First(&track).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		s.itemUnchanged(it.stat.ModTime(), track.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}

	trags, err := tags.New(it.fullPath)
	if err != nil {
		return ErrReadingTags
	}

	genreIDs, err := s.populateGenres(&track, trags)
	if err != nil {
		return fmt.Errorf("populate genres: %w", err)
	}

	// create album and album artist records for first track in album
	if album.TagTitle == "" {
		albumArtist, err := s.populateAlbumArtist(trags)
		if err != nil {
			return fmt.Errorf("populate artist: %w", err)
		}

		albumName := trags.SomeAlbum()
		album.TagTitle = albumName
		album.TagTitleUDec = decoded(albumName)
		album.TagBrainzID = trags.AlbumBrainzID()
		album.TagYear = trags.Year()
		album.TagArtistID = albumArtist.ID

		if err := s.populateAlbumGenres(album, genreIDs); err != nil {
			return fmt.Errorf("populate album genres: %w", err)
		}
	}

	track.Filename = it.filename
	track.FilenameUDec = decoded(it.filename)
	track.Size = int(it.stat.Size())
	track.AlbumID = album.ID
	track.ArtistID = album.TagArtistID

	track.TagTitle = trags.Title()
	track.TagTitleUDec = decoded(trags.Title())
	track.TagTrackArtist = trags.Artist()
	track.TagTrackNumber = trags.TrackNumber()
	track.TagDiscNumber = trags.DiscNumber()
	track.TagBrainzID = trags.BrainzID()

	track.Length = trags.Length()   // these two should be calculated
	track.Bitrate = trags.Bitrate() // ...from the file instead of tags

	if err := s.trTx.Save(&track).Error; err != nil {
		return fmt.Errorf("writing track table: %w", err)
	}
	s.seenTracksNew++

	if err := s.populateTrackGenres(&track, genreIDs); err != nil {
		return fmt.Errorf("populating track genres : %w", err)
	}

	return nil
}

func (s *Scanner) populateAlbumArtist(trags *tags.Tags) (*db.Artist, error) {
	var artist db.Artist
	artistName := trags.SomeAlbumArtist()
	err := s.trTx.
		Where("name=?", artistName).
		Assign(db.Artist{
			Name:     artistName,
			NameUDec: decoded(artistName),
		}).
		FirstOrCreate(&artist).
		Error
	if err != nil {
		return nil, fmt.Errorf("find or create artist: %w", err)
	}
	return &artist, nil
}

func (s *Scanner) populateGenres(track *db.Track, trags *tags.Tags) ([]int, error) {
	var genreIDs []int
	genreNames := strings.Split(trags.SomeGenre(), s.genreSplit)
	for _, genreName := range genreNames {
		genre := &db.Genre{}
		q := s.trTx.FirstOrCreate(genre, db.Genre{
			Name: genreName,
		})
		if err := q.Error; err != nil {
			return nil, err
		}
		genreIDs = append(genreIDs, genre.ID)
	}
	return genreIDs, nil
}

func (s *Scanner) populateTrackGenres(track *db.Track, genreIDs []int) error {
	err := s.trTx.
		Where("track_id=?", track.ID).
		Delete(db.TrackGenre{}).
		Error
	if err != nil {
		return fmt.Errorf("delete old track genre records: %w", err)
	}

	err = s.trTx.InsertBulkLeftMany(
		"track_genres",
		[]string{"track_id", "genre_id"},
		track.ID,
		genreIDs,
	)
	if err != nil {
		return fmt.Errorf("insert bulk track genres: %w", err)
	}
	return nil
}

func (s *Scanner) populateAlbumGenres(album *db.Album, genreIDs []int) error {
	err := s.trTx.
		Where("album_id=?", album.ID).
		Delete(db.AlbumGenre{}).
		Error
	if err != nil {
		return fmt.Errorf("delete old album genre records: %w", err)
	}
	err = s.trTx.InsertBulkLeftMany(
		"album_genres",
		[]string{"album_id", "genre_id"},
		album.ID,
		genreIDs,
	)
	if err != nil {
		return fmt.Errorf("insert bulk album genres: %w", err)
	}
	return nil
}
