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

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"time"

	"github.com/dhowden/tag"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/karrick/godirwalk"

	"github.com/sentriz/gonic/db"
)

var (
	orm *gorm.DB
	tx  *gorm.DB
	// seenPaths is used to keep every path we've seen so that
	// we can remove old tracks, folders, and covers by path when we
	// are in the cleanDatabase stage
	seenPaths = make(map[string]bool)
	// currentDirStack is used for inserting to the folders (subsonic browse
	// by folder) which helps us work out a folder's parent
	currentDirStack = make(dirStack, 0)
	// currentCover because we find a cover anywhere among the tracks during the
	// walk and need a reference to it when we update folder and album records
	// when we exit a folder
	currentCover = &db.Cover{}
	// currentAlbum because we update this record when we exit a folder with
	// our new reference to it's cover
	currentAlbum = &db.Album{}
)

func readTags(fullPath string) (tag.Metadata, error) {
	trackData, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("when tags from disk: %v", err)
	}
	defer trackData.Close()
	tags, err := tag.ReadFrom(trackData)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func updateAlbum(fullPath string, album *db.Album) {
	if currentAlbum.ID != 0 {
		return
	}
	directory, _ := path.Split(fullPath)
	// update album table (the currentAlbum record will be updated when
	// we exit this folder)
	err := tx.Where("path = ?", directory).First(currentAlbum).Error
	if !gorm.IsRecordNotFoundError(err) {
		// we found the record
		// TODO: think about mod time here
		return
	}
	currentAlbum = &db.Album{
		Path:          directory,
		Title:         album.Title,
		AlbumArtistID: album.AlbumArtistID,
		Year:          album.Year,
	}
	tx.Save(currentAlbum)
}

func handleCover(fullPath string, stat os.FileInfo) error {
	modTime := stat.ModTime()
	err := tx.Where("path = ?", fullPath).First(currentCover).Error
	if !gorm.IsRecordNotFoundError(err) &&
		modTime.Before(currentCover.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	image, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("when reading cover: %v", err)
	}
	currentCover = &db.Cover{
		Path:          fullPath,
		Image:         image,
		NewlyInserted: true,
	}
	tx.Save(currentCover)
	return nil
}

func handleFolder(fullPath string, stat os.FileInfo) error {
	// update folder table for browsing by folder
	folder := &db.Folder{}
	defer currentDirStack.Push(folder)
	modTime := stat.ModTime()
	err := tx.Where("path = ?", fullPath).First(folder).Error
	if !gorm.IsRecordNotFoundError(err) &&
		modTime.Before(folder.UpdatedAt) {
		// we found the record but it hasn't changed
		return nil
	}
	_, folderName := path.Split(fullPath)
	folder.Path = fullPath
	folder.ParentID = currentDirStack.PeekID()
	folder.Name = folderName
	tx.Save(folder)
	return nil
}

func handleFolderCompletion(fullPath string, info *godirwalk.Dirent) error {
	currentDir := currentDirStack.Peek()
	defer currentDirStack.Pop()
	var dirShouldSave bool
	if currentAlbum.ID != 0 {
		currentAlbum.CoverID = currentCover.ID
		tx.Save(currentAlbum)
		currentDir.HasTracks = true
		dirShouldSave = true
	}
	if currentCover.NewlyInserted {
		currentDir.CoverID = currentCover.ID
		dirShouldSave = true
	}
	if dirShouldSave {
		tx.Save(currentDir)
	}
	currentCover = &db.Cover{}
	currentAlbum = &db.Album{}
	log.Printf("processed folder `%s`\n", fullPath)
	return nil
}

func handleTrack(fullPath string, stat os.FileInfo, mime, exten string) error {
	//
	// set track basics
	track := &db.Track{}
	modTime := stat.ModTime()
	err := tx.Where("path = ?", fullPath).First(track).Error
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
	track.FolderID = currentDirStack.PeekID()
	//
	// set album artist basics
	albumArtist := &db.AlbumArtist{}
	err = tx.Where("name = ?", tags.AlbumArtist()).
		First(albumArtist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		albumArtist.Name = tags.AlbumArtist()
		tx.Save(albumArtist)
	}
	track.AlbumArtistID = albumArtist.ID
	//
	// set temporary album's basics - will be updated with
	// cover after the tracks inserted when we exit the folder
	updateAlbum(fullPath, &db.Album{
		AlbumArtistID: albumArtist.ID,
		Title:         tags.Album(),
		Year:          tags.Year(),
	})
	//
	// update the track with our new album and finally save
	track.AlbumID = currentAlbum.ID
	tx.Save(track)
	return nil
}

func handleItem(fullPath string, info *godirwalk.Dirent) error {
	// seenPaths = append(seenPaths, fullPath)
	seenPaths[fullPath] = true
	stat, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("error stating: %v", err)
	}
	if info.IsDir() {
		return handleFolder(fullPath, stat)
	}
	if isCover(fullPath) {
		return handleCover(fullPath, stat)
	}
	if mime, exten, ok := isAudio(fullPath); ok {
		return handleTrack(fullPath, stat, mime, exten)
	}
	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <path to music>", os.Args[0])
	}
	orm = db.New()
	orm.SetLogger(log.New(os.Stdout, "gorm ", 0))
	tx = orm.Begin()
	createDatabase()
	startTime := time.Now()
	err := godirwalk.Walk(os.Args[1], &godirwalk.Options{
		Callback:             handleItem,
		PostChildrenCallback: handleFolderCompletion,
		Unsorted:             true,
	})
	if err != nil {
		log.Fatalf("error when walking: %v\n", err)
	}
	log.Printf("scanned in %s\n", time.Since(startTime))
	startTime = time.Now()
	cleanDatabase()
	log.Printf("cleaned in %s\n", time.Since(startTime))
	tx.Commit()
}
