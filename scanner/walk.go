package scanner

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"

	"github.com/sentriz/gonic/model"
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

func (s *Scanner) callbackItem(path string, info *godirwalk.Dirent) error {
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

func (s *Scanner) callbackPost(path string, info *godirwalk.Dirent) error {
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
	s.curAArtist = model.Artist{}
	//
	log.Printf("processed folder `%s`\n", path)
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
	track.TrackArtist = tags.Artist()
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
	track.ArtistID = s.curAArtist.ID
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
	s.curAlbum.ArtistID = s.curAArtist.ID
	s.curAlbum.IsNew = true
	return nil
}
