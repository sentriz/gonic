package scanner

import (
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
	"github.com/pkg/errors"
	"github.com/rainycape/unidecode"

	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/mime"
	"go.senan.xyz/gonic/server/scanner/stack"
	"go.senan.xyz/gonic/server/scanner/tags"
)

var (
	ErrStatingItem = errors.New("stating item")
	ErrReadingTags = errors.New("reading tags")
)

func durSince(t time.Time) time.Duration {
	return time.Since(t).Truncate(10 * time.Microsecond)
}

// decoded converts a string to it's latin equivalent. it will
// be used by the model's *UDec fields, and is only set if it
// differs from the original. the fields are used for searching
func decoded(in string) string {
	if u := unidecode.Unidecode(in); u != in {
		return u
	}
	return ""
}

// isScanning acts as an atomic boolean semaphore. we don't
// want to have more than one scan going on at a time
var isScanning int32

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
	db        *db.DB
	musicPath string
	isFull    bool
	// these two are for the transaction we do for every folder.
	// the boolean is there so we dont begin or commit multiple
	// times in the handle folder or post children callback
	trTx     *gorm.DB
	trTxOpen bool
	// these two are for keeping state between noted in the tree.
	// eg. keep track of a parents folder or the path to a cover
	// we just saw that we need to commit in the post children
	// callback
	curFolders *stack.Stack
	curCover   string
	// then the rest are for stats and cleanup at the very end
	seenTracks    map[int]struct{} // set of p keys
	seenFolders   map[int]struct{} // set of p keys
	seenTracksNew int              // n tracks not seen before
	seenTracksErr int              // n tracks we we couldn't scan
}

func New(musicPath string, db *db.DB) *Scanner {
	return &Scanner{
		db:        db,
		musicPath: musicPath,
	}
}

// ## begin clean funcs
// ## begin clean funcs
// ## begin clean funcs

func (s *Scanner) cleanTracks() (int, error) {
	var previous []int
	var missing []int64
	err := s.db.
		Model(&db.Track{}).
		Pluck("id", &previous).
		Error
	if err != nil {
		return 0, errors.Wrap(err, "plucking ids")
	}
	for _, prev := range previous {
		if _, ok := s.seenTracks[prev]; !ok {
			missing = append(missing, int64(prev))
		}
	}
	err = s.db.WithTxChunked(missing, func(tx *gorm.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Track{}).Error
	})
	return len(missing), err
}

func (s *Scanner) cleanFolders() (int, error) {
	var previous []int
	var missing []int64
	err := s.db.
		Model(&db.Album{}).
		Pluck("id", &previous).
		Error
	if err != nil {
		return 0, errors.Wrap(err, "plucking ids")
	}
	for _, prev := range previous {
		if _, ok := s.seenFolders[prev]; !ok {
			missing = append(missing, int64(prev))
		}
	}
	err = s.db.WithTxChunked(missing, func(tx *gorm.DB, chunk []int64) error {
		return tx.Where(chunk).Delete(&db.Album{}).Error
	})
	return len(missing), err
}

func (s *Scanner) cleanArtists() (int, error) {
	q := s.db.Exec(`
		DELETE FROM artists
		WHERE NOT EXISTS ( SELECT 1 FROM albums
		                   WHERE albums.tag_artist_id=artists.id )
	`)
	return int(q.RowsAffected), q.Error
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
		return errors.New("already scanning")
	}
	unSet := SetScanning()
	defer unSet()
	// reset state vars for the new scan
	s.isFull = opts.IsFull
	s.seenTracks = map[int]struct{}{}
	s.seenFolders = map[int]struct{}{}
	s.curFolders = &stack.Stack{}
	s.seenTracksNew = 0
	s.seenTracksErr = 0
	// ** begin being walking
	log.Println("starting scan")
	start := time.Now()
	err := godirwalk.Walk(s.musicPath, &godirwalk.Options{
		Callback:             s.callbackItem,
		PostChildrenCallback: s.callbackPost,
		Unsorted:             true,
		FollowSymbolicLinks:  true,
		ErrorCallback: func(s string, err error) godirwalk.ErrorAction {
			return
		},
	})
	if err != nil {
		return errors.Wrap(err, "walking filesystem")
	}
	log.Printf("finished scan in %s, +%d/%d tracks (%d err)\n",
		durSince(start),
		s.seenTracksNew,
		len(s.seenTracks),
		s.seenTracksErr,
	)
	// ** begin cleaning
	cleanFuncs := []struct {
		name string
		f    func() (int, error)
	}{
		{name: "tracks", f: s.cleanTracks},
		{name: "folders", f: s.cleanFolders},
		{name: "artists", f: s.cleanArtists},
	}
	for _, clean := range cleanFuncs {
		start = time.Now()
		deleted, _ := clean.f()
		log.Printf("finished clean %s in %s, %d removed",
			clean.name, durSince(start), deleted)
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

var coverFilenames = map[string]struct{}{
	"cover.png":   {},
	"cover.jpg":   {},
	"cover.jpeg":  {},
	"folder.png":  {},
	"folder.jpg":  {},
	"folder.jpeg": {},
	"album.png":   {},
	"album.jpg":   {},
	"album.jpeg":  {},
	"front.png":   {},
	"front.jpg":   {},
	"front.jpeg":  {},
}

// ## begin callbacks
// ## begin callbacks
// ## begin callbacks

func (s *Scanner) callbackItem(fullPath string, info *godirwalk.Dirent) error {
	stat, err := os.Stat(fullPath)
	if err != nil {
		return errors.Wrap(err, "stating")
	}
	relPath, err := filepath.Rel(s.musicPath, fullPath)
	if err != nil {
		return errors.Wrap(err, "getting relative path")
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
		return errors.Wrap(err, "stating link to dir")
	}
	if isDir {
		return s.handleFolder(it)
	}
	lowerFilename := strings.ToLower(filename)
	if _, ok := coverFilenames[lowerFilename]; ok {
		s.curCover = filename
		return nil
	}
	ext := path.Ext(filename)
	if ext == "" {
		return nil
	}
	if _, ok := mime.Types[ext[1:]]; ok {
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
	// begin taking the current folder off the stack and add it's
	// parent, cover that we found, etc.
	folder := s.curFolders.Pop()
	if !folder.ReceivedPaths {
		return nil
	}
	folder.ParentID = s.curFolders.PeekID()
	folder.Cover = s.curCover
	s.db.Save(folder)
	// we only log changed folders
	log.Printf("processed folder `%s`\n",
		path.Join(folder.LeftPath, folder.RightPath))
	return nil
}

// ## begin handlers
// ## begin handlers
// ## begin handlers

func (s *Scanner) itemUnchanged(stat, updated time.Time) bool {
	if s.isFull {
		return false
	}
	return stat.Before(updated)
}

func (s *Scanner) handleFolder(it *item) error {
	if s.trTxOpen {
		// a transaction still being open when we handle a folder can
		// happen if there is a folder that contains /both/ tracks and
		// sub folders
		s.trTx.Commit()
		s.trTxOpen = false
	}
	folder := &db.Album{}
	defer func() {
		// folder's id will come from early return
		// or save at the end
		s.seenFolders[folder.ID] = struct{}{}
		s.curFolders.Push(folder)
	}()
	err := s.db.
		Select("id, updated_at").
		Where(db.Album{
			LeftPath:  it.directory,
			RightPath: it.filename,
		}).
		First(folder).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		s.itemUnchanged(it.stat.ModTime(), folder.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	folder.LeftPath = it.directory
	folder.RightPath = it.filename
	folder.RightPathUDec = decoded(it.filename)
	folder.ModifiedAt = it.stat.ModTime()
	s.db.Save(folder)
	folder.ReceivedPaths = true
	return nil
}

func (s *Scanner) handleTrack(it *item) error {
	if !s.trTxOpen {
		s.trTx = s.db.Begin()
		s.trTxOpen = true
	}
	// ** begin set track basics
	track := &db.Track{}
	defer func() {
		// folder's id will come from early return
		// or save at the end
		s.seenTracks[track.ID] = struct{}{}
	}()
	err := s.trTx.
		Select("id, updated_at").
		Where(db.Track{
			AlbumID:  s.curFolders.PeekID(),
			Filename: it.filename,
		}).
		First(track).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		s.itemUnchanged(it.stat.ModTime(), track.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	track.Filename = it.filename
	track.FilenameUDec = decoded(it.filename)
	track.Size = int(it.stat.Size())
	track.AlbumID = s.curFolders.PeekID()
	trTags, err := tags.New(it.fullPath)
	if err != nil {
		// not returning the error here because we don't want
		// the entire walk to stop if we can't read the tags
		// of a single file
		log.Printf("error reading tags `%s`: %v", it.relPath, err)
		s.seenTracksErr++
		return nil
	}
	track.TagTitle = trTags.Title()
	track.TagTitleUDec = decoded(trTags.Title())
	track.TagTrackArtist = trTags.Artist()
	track.TagTrackNumber = trTags.TrackNumber()
	track.TagDiscNumber = trTags.DiscNumber()
	track.TagBrainzID = trTags.BrainzID()
	track.Length = trTags.Length()   // these two should be calculated
	track.Bitrate = trTags.Bitrate() // ...from the file instead of tags
	// ** begin set album artist basics
	artistName := func() string {
		if r := trTags.AlbumArtist(); r != "" {
			return r
		}
		if r := trTags.Artist(); r != "" {
			return r
		}
		return "Unknown Artist"
	}()
	artist := &db.Artist{}
	err = s.trTx.
		Select("id").
		Where("name=?", artistName).
		First(artist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		artist.Name = artistName
		artist.NameUDec = decoded(artistName)
		s.trTx.Save(artist)
	}
	track.ArtistID = artist.ID
	// ** begin set genre
	genreName := func() string {
		if r := trTags.Genre(); r != "" {
			return r
		}
		return "Unknown Genre"
	}()
	genre := &db.Genre{}
	err = s.trTx.
		Select("id").
		Where("name=?", genreName).
		First(genre).
		Error
	if gorm.IsRecordNotFoundError(err) {
		genre.Name = genreName
		s.trTx.Save(genre)
	}
	track.TagGenreID = genre.ID
	// ** begin save the track
	s.trTx.Save(track)
	s.seenTracksNew++
	// ** begin set album if this is the first track in the folder
	folder := s.curFolders.Peek()
	if !folder.ReceivedPaths || folder.ReceivedTags {
		// the folder hasn't been modified or already has it's tags
		return nil
	}
	folder.TagTitle = trTags.Album()
	folder.TagTitleUDec = decoded(trTags.Album())
	folder.TagBrainzID = trTags.AlbumBrainzID()
	folder.TagYear = trTags.Year()
	folder.TagArtistID = artist.ID
	folder.TagGenreID = genre.ID
	folder.ReceivedTags = true
	return nil
}
