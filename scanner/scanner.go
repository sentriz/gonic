package scanner

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
	seenTracks map[int]struct{}
	curFolders folderStack
	curCover   string
}

func New(db *gorm.DB, musicPath string) *Scanner {
	return &Scanner{
		db:         db,
		musicPath:  musicPath,
		seenTracks: make(map[int]struct{}),
		curFolders: make(folderStack, 0),
	}
}

func (s *Scanner) curFolder() *model.Album {
	return s.curFolders.Peek()
}

func (s *Scanner) curFolderID() int {
	peek := s.curFolders.Peek()
	if peek == nil {
		return 0
	}
	return peek.ID
}

func (s *Scanner) MigrateDB() error {
	defer logElapsed(time.Now(), "migrating database")
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
	var tracks []*model.Track
	err := s.tx.
		Select("id").
		Find(&tracks).
		Error
	if err != nil {
		return errors.Wrap(err, "scanning tracks")
	}
	var deleted int
	for _, track := range tracks {
		_, ok := s.seenTracks[track.ID]
		if !ok {
			s.tx.Delete(track)
			deleted++
		}
	}
	log.Printf("removed %d tracks\n", deleted)
	return nil
}
