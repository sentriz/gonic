package scanner

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"
	"github.com/rainycape/unidecode"

	"github.com/sentriz/gonic/mime"
	"github.com/sentriz/gonic/model"
	"github.com/sentriz/gonic/scanner/stack"
	"github.com/sentriz/gonic/scanner/tags"
)

// IsScanning acts as an atomic boolean semaphore. we don't
// want to have more than one scan going on at a time
var IsScanning int32

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

// decoded converts a string to it's latin equivalent. it
// will be used by the model's *UDec fields, and is only set
// if it differs from the original.
// the fields are used for searching
func decoded(in string) string {
	result := unidecode.Unidecode(in)
	if result == in {
		return ""
	}
	return result
}

type Scanner struct {
	db, tx        *gorm.DB
	musicPath     string
	seenTracks    map[int]struct{}
	seenFolders   map[int]struct{}
	seenTracksNew int
	seenTracksErr int
	curFolders    *stack.Stack
	curCover      string
}

func New(db *gorm.DB, musicPath string) *Scanner {
	return &Scanner{
		db:          db,
		musicPath:   musicPath,
		seenTracks:  make(map[int]struct{}),
		seenFolders: make(map[int]struct{}),
		curFolders:  &stack.Stack{},
	}
}

func (s *Scanner) MigrateDB() error {
	s.tx = s.db.Begin()
	defer s.tx.Commit()
	s.tx.AutoMigrate(
		model.Artist{},
		model.Track{},
		model.User{},
		model.Setting{},
		model.Play{},
		model.Album{},
	)
	s.tx.FirstOrCreate(&model.User{}, model.User{
		Name:     "admin",
		Password: "admin",
		IsAdmin:  true,
	})
	log.Printf("finished migrate")
	return nil
}

func (s *Scanner) Start() error {
	if atomic.LoadInt32(&IsScanning) == 1 {
		return errors.New("already scanning")
	}
	atomic.StoreInt32(&IsScanning, 1)
	defer atomic.StoreInt32(&IsScanning, 0)
	s.tx = s.db.Begin()
	defer s.tx.Commit()
	//
	// being walking
	start := time.Now()
	err := godirwalk.Walk(s.musicPath, &godirwalk.Options{
		Callback:             s.callbackItem,
		PostChildrenCallback: s.callbackPost,
		Unsorted:             true,
	})
	if err != nil {
		return errors.Wrap(err, "walking filesystem")
	}
	log.Printf("finished scan in %s, +%d/%d tracks (%d err)\n",
		time.Since(start),
		s.seenTracksNew,
		len(s.seenTracks),
		s.seenTracksErr,
	)
	//
	// begin cleaning
	start = time.Now()
	var tracks []*model.Track
	err = s.tx.
		Select("id").
		Find(&tracks).
		Error
	if err != nil {
		return errors.Wrap(err, "scanning tracks")
	}
	// delete tracks not on filesystem
	var deleted uint
	for _, track := range tracks {
		_, ok := s.seenTracks[track.ID]
		if !ok {
			s.tx.Delete(track)
			deleted++
		}
	}
	// delete folders not on filesystem
	var folders []*model.Album
	s.tx.
		Select("id").
		Find(&folders)
	for _, folder := range folders {
		_, ok := s.seenFolders[folder.ID]
		if !ok {
			s.tx.Delete(folder)
		}
	}
	// then, delete albums without tracks
	s.tx.Exec(`
        DELETE FROM albums
        WHERE tag_artist_id NOT NULL
        AND NOT EXISTS (SELECT 1 FROM tracks
                        WHERE tracks.album_id = albums.id)
	`)
	// then, delete artists without albums
	s.tx.Exec(`
        DELETE FROM artists
        WHERE NOT EXISTS (SELECT 1 from albums
                          WHERE albums.tag_artist_id = artists.id)
	`)
	log.Printf("finished clean in %s, -%d tracks\n",
		time.Since(start),
		deleted,
	)
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
	if info.IsDir() {
		return s.handleFolder(it)
	}
	lowerFilename := strings.ToLower(filename)
	if _, ok := coverFilenames[lowerFilename]; ok {
		s.curCover = filename
		return nil
	}
	ext := path.Ext(filename)[1:]
	if _, ok := mime.Types[ext]; ok {
		return s.handleTrack(it)
	}
	return nil
}

func (s *Scanner) callbackPost(fullPath string, info *godirwalk.Dirent) error {
	folder := s.curFolders.Pop()
	if folder.ReceivedPaths {
		folder.ParentID = s.curFolders.PeekID()
		folder.Cover = s.curCover
		s.tx.Save(folder)
		log.Printf("processed folder `%s`\n",
			path.Join(folder.LeftPath, folder.RightPath))
	}
	s.curCover = ""
	return nil
}

func (s *Scanner) handleFolder(it *item) error {
	folder := &model.Album{}
	defer func() {
		// folder's id will come from early return
		// or save at the end
		s.seenFolders[folder.ID] = struct{}{}
		s.curFolders.Push(folder)
	}()
	err := s.tx.
		Where(model.Album{
			LeftPath:  it.directory,
			RightPath: it.filename,
		}).
		First(folder).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(folder.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	folder.LeftPath = it.directory
	folder.RightPath = it.filename
	folder.RightPathUDec = decoded(it.filename)
	s.tx.Save(folder)
	folder.ReceivedPaths = true
	return nil
}

func (s *Scanner) handleTrack(it *item) error {
	//
	// set track basics
	track := &model.Track{}
	err := s.tx.
		Where(model.Track{
			AlbumID:  s.curFolders.PeekID(),
			Filename: it.filename,
		}).
		First(track).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(track.UpdatedAt) {
		// we found the record but it hasn't changed
		s.seenTracks[track.ID] = struct{}{}
		return nil
	}
	track.Filename = it.filename
	track.FilenameUDec = decoded(it.filename)
	track.Size = int(it.stat.Size())
	track.AlbumID = s.curFolders.PeekID()
	trTags, err := tags.New(it.fullPath)
	if err != nil {
		// not returning the error here because we don't
		// want the entire walk to stop if we can't read
		// the tags of a single file
		log.Printf("error reading tags `%s`: %v", it.relPath, err)
		s.seenTracksErr++
		return nil
	}
	track.TagTitle = trTags.Title()
	track.TagTitleUDec = decoded(trTags.Title())
	track.TagTrackArtist = trTags.Artist()
	track.TagTrackNumber = trTags.TrackNumber()
	track.TagDiscNumber = trTags.DiscNumber()
	track.Length = trTags.Length()   // these two should be calculated
	track.Bitrate = trTags.Bitrate() // from the file instead of tags
	//
	// set album artist basics
	artist := &model.Artist{}
	artistName := func() string {
		if ret := trTags.AlbumArtist(); ret != "" {
			return ret
		}
		if ret := trTags.Artist(); ret != "" {
			return ret
		}
		return "Unknown Artist"
	}()
	err = s.tx.
		Where("name = ?", artistName).
		First(artist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		artist.Name = artistName
		artist.NameUDec = decoded(artistName)
		s.tx.Save(artist)
	}
	track.ArtistID = artist.ID
	s.tx.Save(track)
	s.seenTracks[track.ID] = struct{}{}
	s.seenTracksNew++
	//
	// set album if this is the first track in the folder
	folder := s.curFolders.Peek()
	if !folder.ReceivedPaths || folder.ReceivedTags {
		// the folder hasn't been modified or already has it's tags
		return nil
	}
	folder.TagTitle = trTags.Album()
	folder.TagTitleUDec = decoded(trTags.Album())
	folder.TagYear = trTags.Year()
	folder.TagArtistID = artist.ID
	folder.ReceivedTags = true
	return nil
}
