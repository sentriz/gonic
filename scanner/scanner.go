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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
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

type Scanner struct {
	db          *gorm.DB
	tx          *gorm.DB
	musicPath   string
	seenPaths   map[string]bool
	folderCount uint
	curFolders  folderStack
	curTracks   []model.Track
	curCover    model.Cover
	curAlbum    model.Album
	curAArtist  model.AlbumArtist
}

func New(db *gorm.DB, musicPath string) *Scanner {
	return &Scanner{
		db:         db,
		musicPath:  musicPath,
		seenPaths:  make(map[string]bool),
		curFolders: make(folderStack, 0),
		curTracks:  make([]model.Track, 0),
		curCover:   model.Cover{},
		curAlbum:   model.Album{},
		curAArtist: model.AlbumArtist{},
	}
}

func (s *Scanner) handleCover(fullPath string, stat os.FileInfo) error {
	err := s.tx.
		Where("path = ?", fullPath).
		First(&s.curCover).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		stat.ModTime().Before(s.curCover.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	s.curCover.Path = fullPath
	image, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return errors.Wrap(err, "reading cover")
	}
	s.curCover.Image = image
	s.curCover.IsNew = true
	return nil
}

func (s *Scanner) handleFolder(fullPath string, stat os.FileInfo) error {
	// TODO:
	var folder model.Folder
	err := s.tx.
		Where("path = ?", fullPath).
		First(&folder).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		stat.ModTime().Before(folder.UpdatedAt) {
		// we found the record but it hasn't changed
		s.curFolders.Push(folder)
		return nil
	}
	folder.Path = fullPath
	folder.Name = stat.Name()
	s.tx.Save(&folder)
	folder.IsNew = true
	s.curFolders.Push(folder)
	return nil
}

func (s *Scanner) handleFolderCompletion(fullPath string, info *godirwalk.Dirent) error {
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
	log.Printf("processed folder `%s`\n", fullPath)
	return nil
}

func (s *Scanner) handleTrack(fullPath string, stat os.FileInfo, mime, exten string) error {
	//
	// set track basics
	track := model.Track{}
	modTime := stat.ModTime()
	err := s.tx.
		Where("path = ?", fullPath).
		First(&track).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		modTime.Before(track.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	tags, err := readTags(fullPath)
	if err != nil {
		return fmt.Errorf("when reading tags: %v", err)
	}
	trackNumber, totalTracks := tags.Track()
	discNumber, totalDiscs := tags.Disc()
	track.Path = fullPath
	track.Title = tags.Title()
	track.Artist = tags.Artist()
	track.DiscNumber = discNumber
	track.TotalDiscs = totalDiscs
	track.TotalTracks = totalTracks
	track.TrackNumber = trackNumber
	track.Year = tags.Year()
	track.Suffix = exten
	track.ContentType = mime
	track.Size = int(stat.Size())
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
	directory, _ := path.Split(fullPath)
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

func (s *Scanner) handleItem(fullPath string, info *godirwalk.Dirent) error {
	s.seenPaths[fullPath] = true
	stat, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("error stating: %v", err)
	}
	if info.IsDir() {
		return s.handleFolder(fullPath, stat)
	}
	if isCover(fullPath) {
		return s.handleCover(fullPath, stat)
	}
	if mime, exten, ok := isAudio(fullPath); ok {
		return s.handleTrack(fullPath, stat, mime, exten)
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
	defer logElapsed(time.Now(), "scanning")
	s.db.Exec("PRAGMA foreign_keys = ON")
	s.tx = s.db.Begin()
	defer s.tx.Commit()
	//
	// start scan logic
	err := godirwalk.Walk(s.musicPath, &godirwalk.Options{
		Callback:             s.handleItem,
		PostChildrenCallback: s.handleFolderCompletion,
		Unsorted:             true,
	})
	if err != nil {
		return errors.Wrap(err, "walking filesystem")
	}
	////
	//// start cleaning logic
	//log.Println("cleaning database")
	//var tracks []*model.Track
	//s.tx.Select("id, path").Find(&tracks)
	//for _, track := range tracks {
	//	_, ok := s.seenPaths[track.Path]
	//	if ok {
	//		continue
	//	}
	//	s.tx.Delete(&track)
	//	log.Println("removed", track.Path)
	//}
	//// delete albums without tracks
	//s.tx.Exec(`
	//DELETE FROM albums
	//WHERE  (SELECT count(id)
	//FROM   tracks
	//WHERE  album_id = albums.id) = 0;
	//`)
	//// delete artists without tracks
	//s.tx.Exec(`
	//DELETE FROM album_artists
	//WHERE  (SELECT count(id)
	//FROM   albums
	//WHERE  album_artist_id = album_artists.id) = 0;
	//`)
	return nil
}
