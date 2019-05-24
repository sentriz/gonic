package scanner

// Album  -> needs a  CoverID
//        -> needs a  FolderID (American Football)
// Folder -> needs a  CoverID
//        -> needs a  ParentID
// Track  -> needs an AlbumID
//        -> needs a  FolderID

import (
	"log"
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

func (s *Scanner) Start() error {
	if atomic.LoadInt32(&IsScanning) == 1 {
		return errors.New("already scanning")
	}
	atomic.StoreInt32(&IsScanning, 1)
	defer atomic.StoreInt32(&IsScanning, 0)
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

func logElapsed(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("finished %s in %s\n", name, elapsed)
}

func (s *Scanner) startScan() error {
	defer logElapsed(time.Now(), "scanning")
	err := godirwalk.Walk(s.musicPath, &godirwalk.Options{
		Callback:             s.callbackItem,
		PostChildrenCallback: s.callbackPost,
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
