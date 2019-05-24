package scanner

// this scanner tries to scan with a single unsorted walk of the music
// directory - which means you can come across the cover of an album/folder
// before the tracks (and therefore the album) which is an issue because
// when inserting into the album table, we need a reference to the cover.
// to solve this we're using godirwalk's PostChildrenCallback and some globals
//
// Album  -> needs a  CoverID
//        -> needs a  FolderID (American Football)
// Folder -> needs a  CoverID
//        -> needs a  ParentID
// Track  -> needs an AlbumID
//        -> needs a  FolderID

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"

	"github.com/sentriz/gonic/model"
)

var (
	IsScanning int32
)

type trackItem struct {
	mime string
	ext  string
}

type item struct {
	path    string
	relPath string
	stat    os.FileInfo
	track   *trackItem
}

type Scanner struct {
	db, tx     *gorm.DB
	musicPath  string
	seenTracks map[string]bool
	curFolders folderStack
	curTracks  []model.Track
	curCover   model.Cover
	curAlbum   model.Album
	curAArtist model.AlbumArtist
}

func New(db *gorm.DB, musicPath string) *Scanner {
	return &Scanner{
		db:         db,
		musicPath:  musicPath,
		seenTracks: make(map[string]bool),
		curFolders: make(folderStack, 0),
		curTracks:  make([]model.Track, 0),
		curCover:   model.Cover{},
		curAlbum:   model.Album{},
		curAArtist: model.AlbumArtist{},
	}
}

func (s *Scanner) handleCover(it *item) error {
	err := s.tx.
		Where("path = ?", it.relPath).
		First(&s.curCover).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(s.curCover.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	s.curCover.Path = it.relPath
	image, err := ioutil.ReadFile(it.path)
	if err != nil {
		return errors.Wrap(err, "reading cover")
	}
	s.curCover.Image = image
	s.curCover.IsNew = true
	return nil
}

func (s *Scanner) handleFolder(it *item) error {
	// TODO:
	var folder model.Folder
	err := s.tx.
		Where("path = ?", it.relPath).
		First(&folder).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(folder.UpdatedAt) {
		// we found the record but it hasn't changed
		s.curFolders.Push(folder)
		return nil
	}
	folder.Path = it.relPath
	folder.Name = it.stat.Name()
	s.tx.Save(&folder)
	folder.IsNew = true
	s.curFolders.Push(folder)
	return nil
}

func (s *Scanner) handleTrack(it *item) error {
	//
	// set track basics
	track := model.Track{}
	err := s.tx.
		Where("path = ?", it.relPath).
		First(&track).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(track.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	tags, err := readTags(it.path)
	if err != nil {
		return errors.Wrap(err, "reading tags")
	}
	trackNumber, totalTracks := tags.Track()
	discNumber, totalDiscs := tags.Disc()
	track.DiscNumber = discNumber
	track.TotalDiscs = totalDiscs
	track.TotalTracks = totalTracks
	track.TrackNumber = trackNumber
	track.Path = it.relPath
	track.Suffix = it.track.ext
	track.ContentType = it.track.mime
	track.Size = int(it.stat.Size())
	track.Title = tags.Title()
	track.Artist = tags.Artist()
	track.Year = tags.Year()
	track.FolderID = s.curFolders.PeekID()
	//
	// set album artist basics
	err = s.tx.Where("name = ?", tags.AlbumArtist()).
		First(&s.curAArtist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		s.curAArtist.Name = tags.AlbumArtist()
		s.tx.Save(&s.curAArtist)
	}
	track.AlbumArtistID = s.curAArtist.ID
	//
	// set album if this is the first track in the folder
	if len(s.curTracks) > 0 {
		s.curTracks = append(s.curTracks, track)
		return nil
	}
	s.curTracks = append(s.curTracks, track)
	//
	directory, _ := path.Split(it.relPath)
	err = s.tx.
		Where("path = ?", directory).
		First(&s.curAlbum).
		Error
	if !gorm.IsRecordNotFoundError(err) {
		// we found the record
		return nil
	}
	s.curAlbum.Path = directory
	s.curAlbum.Title = tags.Album()
	s.curAlbum.Year = tags.Year()
	s.curAlbum.AlbumArtistID = s.curAArtist.ID
	s.curAlbum.IsNew = true
	return nil
}

func (s *Scanner) handleFolderCompletion(path string, info *godirwalk.Dirent) error {
	// in general in this function - if a model is not nil, then it
	// has at least been looked up. if it has a id of 0, then it is
	// a new record and needs to be inserted
	if s.curCover.IsNew {
		s.tx.Save(&s.curCover)
	}
	if s.curAlbum.IsNew {
		s.curAlbum.CoverID = s.curCover.ID
		s.tx.Save(&s.curAlbum)
	}
	folder := s.curFolders.Pop()
	if folder.IsNew {
		folder.ParentID = s.curFolders.PeekID()
		folder.CoverID = s.curCover.ID
		folder.HasTracks = len(s.curTracks) > 1
		s.tx.Save(&folder)
	}
	for _, t := range s.curTracks {
		t.FolderID = folder.ID
		t.AlbumID = s.curAlbum.ID
		s.tx.Save(&t)
	}
	//
	s.curTracks = make([]model.Track, 0)
	s.curCover = model.Cover{}
	s.curAlbum = model.Album{}
	s.curAArtist = model.AlbumArtist{}
	//
	log.Printf("processed folder `%s`\n", path)
	return nil
}

func (s *Scanner) handleItem(path string, info *godirwalk.Dirent) error {
	stat, err := os.Stat(path)
	if err != nil {
		return errors.Wrap(err, "stating")
	}
	relPath, err := filepath.Rel(s.musicPath, path)
	if err != nil {
		return errors.Wrap(err, "getting relative path")
	}
	it := &item{
		path:    path,
		relPath: relPath,
		stat:    stat,
	}
	if info.IsDir() {
		return s.handleFolder(it)
	}
	if isCover(path) {
		return s.handleCover(it)
	}
	if mime, ext, ok := isTrack(path); ok {
		s.seenTracks[relPath] = true
		it.track = &trackItem{mime: mime, ext: ext}
		return s.handleTrack(it)
	}
	return nil
}

func (s *Scanner) startScan() error {
	defer logElapsed(time.Now(), "scanning")
	err := godirwalk.Walk(s.musicPath, &godirwalk.Options{
		Callback:             s.handleItem,
		PostChildrenCallback: s.handleFolderCompletion,
		Unsorted:             true,
	})
	if err != nil {
		return errors.Wrap(err, "walking filesystem")
	}
	return nil
}

func (s *Scanner) startClean() error {
	defer logElapsed(time.Now(), "cleaning database")
	var tracks []model.Track
	s.tx.
		Select("id, path").
		Find(&tracks)
	for _, track := range tracks {
		_, ok := s.seenTracks[track.Path]
		if ok {
			continue
		}
		s.tx.Delete(&track)
		log.Println("removed track", track.Path)
	}
	return nil
}

func (s *Scanner) MigrateDB() error {
	defer logElapsed(time.Now(), "migrating database")
	s.db.Exec("PRAGMA foreign_keys = ON")
	s.tx = s.db.Begin()
	defer s.tx.Commit()
	s.tx.AutoMigrate(
		model.Album{},
		model.AlbumArtist{},
		model.Track{},
		model.Cover{},
		model.User{},
		model.Setting{},
		model.Play{},
		model.Folder{},
	)
	s.tx.FirstOrCreate(&model.User{}, model.User{
		Name:     "admin",
		Password: "admin",
		IsAdmin:  true,
	})
	return nil
}

func (s *Scanner) Start() error {
	if atomic.LoadInt32(&IsScanning) == 1 {
		return errors.New("already scanning")
	}
	atomic.StoreInt32(&IsScanning, 1)
	defer atomic.StoreInt32(&IsScanning, 0)
	s.db.Exec("PRAGMA foreign_keys = ON")
	s.tx = s.db.Begin()
	defer s.tx.Commit()
	if err := s.startScan(); err != nil {
		return errors.Wrap(err, "start scan")
	}
	if err := s.startClean(); err != nil {
		return errors.Wrap(err, "start clean")
	}
	return nil
}
