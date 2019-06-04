package scanner

import (
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/jinzhu/gorm"
	"github.com/karrick/godirwalk"
	"github.com/pkg/errors"

	"github.com/sentriz/gonic/model"
)

type item struct {
	//
	// common
	fullPath string
	relPath  string
	filename string
	stat     os.FileInfo
	//
	// track only
	ext  string
	mime string
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
	_, filename := path.Split(relPath)
	it := &item{
		fullPath: fullPath,
		relPath:  relPath,
		filename: filename,
		stat:     stat,
	}
	if info.IsDir() {
		return s.handleFolder(it)
	}
	if _, ok := coverFilenames[filename]; ok {
		s.curCover = filename
		return nil
	}
	ext := path.Ext(filename)[1:]
	if mime, ok := mimeTypes[ext]; ok {
		it.ext = ext
		it.mime = mime
		return s.handleTrack(it)
	}
	return nil
}

func (s *Scanner) callbackPost(fullPath string, info *godirwalk.Dirent) error {
	folder := s.curFolders.Pop()
	if folder.IsNew {
		folder.ParentID = s.curFolderID()
		folder.Cover = s.curCover
		s.tx.Save(&folder)
	}
	s.curCover = ""
	log.Printf("processed folder `%s`\n", fullPath)
	return nil
}

func (s *Scanner) handleFolder(it *item) error {
	var folder model.Folder
	err := s.tx.
		Where("path = ?", it.relPath).
		First(&folder).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(folder.UpdatedAt) {
		// we found the record but it hasn't changed
		s.curFolders.Push(&folder)
		return nil
	}
	folder.Path = it.relPath
	s.tx.Save(&folder)
	folder.IsNew = true
	s.curFolders.Push(&folder)
	return nil
}

func (s *Scanner) handleTrack(it *item) error {
	//
	// set track basics
	var track model.Track
	err := s.tx.
		Where(model.Track{
			FolderID: s.curFolderID(),
			Filename: it.filename,
		}).
		First(&track).
		Error
	if !gorm.IsRecordNotFoundError(err) &&
		it.stat.ModTime().Before(track.UpdatedAt) {
		s.seenTracks[track.ID] = struct{}{}
		// we found the record but it hasn't changed
		return nil
	}
	track.Filename = it.filename
	track.ContentType = it.mime
	track.Size = int(it.stat.Size())
	track.FolderID = s.curFolderID()
	track.Duration = -1
	track.Bitrate = -1
	tags, err := readTags(it.fullPath)
	if err != nil {
		return errors.Wrap(err, "reading tags")
	}
	trackNumber, totalTracks := tags.Track()
	discNumber, totalDiscs := tags.Disc()
	track.TagDiscNumber = discNumber
	track.TagTotalDiscs = totalDiscs
	track.TagTotalTracks = totalTracks
	track.TagTrackNumber = trackNumber
	track.TagTitle = tags.Title()
	track.TagTrackArtist = tags.Artist()
	track.TagYear = tags.Year()
	//
	// set album artist basics
	var artist model.Artist
	err = s.tx.Where("name = ?", tags.AlbumArtist()).
		First(&artist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		artist.Name = tags.AlbumArtist()
		s.tx.Save(&artist)
	}
	track.ArtistID = artist.ID
	s.tx.Save(&track)
	s.seenTracks[track.ID] = struct{}{}
	//
	// set album if this is the first track in the folder
	if !s.curFolder().IsNew {
		return nil
	}
	s.curFolder().AlbumTitle = tags.Album()
	s.curFolder().AlbumYear = tags.Year()
	s.curFolder().AlbumArtistID = artist.ID
	return nil
}
