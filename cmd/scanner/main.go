package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/model"

	"github.com/dhowden/tag"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/karrick/godirwalk"
)

var (
	orm             *gorm.DB
	tx              *gorm.DB
	cLastAlbum      = &lastAlbum{}
	audioExtensions = map[string]bool{
		".mp3":  true,
		".flac": true,
		".aac":  true,
		".m4a":  true,
	}
	coverFilenames = map[string]bool{
		"cover.png":   true,
		"cover.jpg":   true,
		"cover.jpeg":  true,
		"folder.png":  true,
		"folder.jpg":  true,
		"folder.jpeg": true,
		"album.png":   true,
		"album.jpg":   true,
		"album.jpeg":  true,
		"front.png":   true,
		"front.jpg":   true,
		"front.jpeg":  true,
	}
)

type lastAlbum struct {
	coverModTime time.Time // 1st needed for cover insertion
	coverPath    string    // 2rd needed for cover insertion
	id           string    // 3nd needed for cover insertion
}

func (l *lastAlbum) isEmpty() bool {
	return l.coverPath == ""
}

func isAudio(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := audioExtensions[ext]
	return ok
}

func isCover(filename string) bool {
	_, ok := coverFilenames[strings.ToLower(filename)]
	return ok
}

func readTags(fullPath string) (tag.Metadata, error) {
	trackData, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("when tags from disk: %v\n", err)
	}
	defer trackData.Close()
	tags, err := tag.ReadFrom(trackData)
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func handleFolderCompletion(fullPath string, info *godirwalk.Dirent) error {
	log.Printf("processed folder `%s`\n", fullPath)
	if cLastAlbum.isEmpty() {
		return nil
	}
	cover := model.Cover{
		Path: cLastAlbum.coverPath,
	}
	err := tx.Where(cover).First(&cover).Error
	if !gorm.IsRecordNotFoundError(err) &&
		!cLastAlbum.coverModTime.After(cover.UpdatedAt) {
		return nil
	}
	image, err := ioutil.ReadFile(cLastAlbum.coverPath)
	if err != nil {
		return fmt.Errorf("when reading cover: %v\n", err)
	}
	cover.Image = image
	cover.AlbumID = cLastAlbum.id
	tx.Save(&cover)
	cLastAlbum = &lastAlbum{}
	return nil
}

func processFile(fullPath string, info *godirwalk.Dirent) error {
	if info.IsDir() {
		return nil
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("when stating file: %v\n", err)
	}
	modTime := stat.ModTime()
	_, filename := path.Split(fullPath)
	if isCover(filename) {
		cLastAlbum.coverModTime = modTime // 1st needed for cover insertion
		cLastAlbum.coverPath = fullPath   // 2nd needed for cover insertion
		return nil
	}
	if !isAudio(filename) {
		return nil
	}
	// set track basics
	track := model.Track{
		Path: fullPath,
	}
	err = tx.Where(track).First(&track).Error
	if !gorm.IsRecordNotFoundError(err) && !modTime.After(track.UpdatedAt) {
		return nil
	}
	tags, err := readTags(fullPath)
	if err != nil {
		return fmt.Errorf("when reading tags: %v\n", err)
	}
	trackNumber, totalTracks := tags.Track()
	discNumber, TotalDiscs := tags.Disc()
	track.Path = fullPath
	track.Title = tags.Title()
	track.DiscNumber = discNumber
	track.TotalDiscs = TotalDiscs
	track.TotalTracks = totalTracks
	track.TrackNumber = trackNumber
	track.Year = tags.Year()
	// set artist
	artist := model.Artist{
		Name: tags.AlbumArtist(),
	}
	err = tx.Where(artist).First(&artist).Error
	if gorm.IsRecordNotFoundError(err) {
		artist.Name = tags.AlbumArtist()
		tx.Save(&artist)
	}
	track.ArtistID = artist.ID
	// set album
	album := model.Album{
		ArtistID: artist.ID,
		Title:    tags.Album(),
	}
	err = tx.Where(album).First(&album).Error
	if gorm.IsRecordNotFoundError(err) {
		album.Title = tags.Album()
		album.ArtistID = artist.ID
		tx.Save(&album)
	}
	track.AlbumID = album.ID
	// set the _3rd_ variable for cover insertion.
	// it will be used by the `handleFolderCompletion` function
	cLastAlbum.id = album.ID
	// save track
	tx.Save(&track)
	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("usage: %s <path to music>", os.Args[0])
	}
	orm = db.New()
	orm.SetLogger(log.New(os.Stdout, "gorm ", 0))
	orm.AutoMigrate(
		&model.Album{},
		&model.Artist{},
		&model.Track{},
		&model.Cover{},
	)
	startTime := time.Now()
	tx = orm.Begin()
	err := godirwalk.Walk(os.Args[1], &godirwalk.Options{
		Callback:             processFile,
		PostChildrenCallback: handleFolderCompletion,
		Unsorted:             true,
	})
	if err != nil {
		log.Fatalf("error when walking: %v\n", err)
	}
	tx.Commit()
	log.Printf("scanned in %s\n", time.Since(startTime))
}
