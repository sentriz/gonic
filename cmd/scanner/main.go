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

	"github.com/dhowden/tag"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/karrick/godirwalk"
)

var (
	orm             *gorm.DB
	tx              *gorm.DB
	cLastAlbum      = &lastAlbum{}
	audioExtensions = map[string]string{
		"mp3":  "audio/mpeg",
		"flac": "audio/x-flac",
		"aac":  "audio/x-aac",
		"m4a":  "audio/m4a",
		"ogg":  "audio/ogg",
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
	id           uint      // 3nd needed for cover insertion
}

func (l *lastAlbum) isEmpty() bool {
	return l.coverPath == ""
}

func isCover(filename string) bool {
	_, ok := coverFilenames[strings.ToLower(filename)]
	return ok
}

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

func handleFolderCompletion(fullPath string, info *godirwalk.Dirent) error {
	log.Printf("processed folder `%s`\n", fullPath)
	if cLastAlbum.isEmpty() {
		return nil
	}
	cover := db.Cover{
		Path: cLastAlbum.coverPath,
	}
	err := tx.Where(cover).First(&cover).Error // TODO: swap
	if !gorm.IsRecordNotFoundError(err) &&
		!cLastAlbum.coverModTime.After(cover.UpdatedAt) {
		return nil
	}
	image, err := ioutil.ReadFile(cLastAlbum.coverPath)
	if err != nil {
		return fmt.Errorf("when reading cover: %v", err)
	}
	cover.Image = image
	cover.AlbumID = cLastAlbum.id
	tx.Save(&cover)
	cLastAlbum = &lastAlbum{}
	return nil
}

func handleFile(fullPath string, info *godirwalk.Dirent) error {
	if info.IsDir() {
		return nil
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("when stating file: %v", err)
	}
	modTime := stat.ModTime()
	_, filename := path.Split(fullPath)
	if isCover(filename) {
		cLastAlbum.coverModTime = modTime // 1st needed for cover insertion
		cLastAlbum.coverPath = fullPath   // 2nd needed for cover insertion
		return nil
	}
	longExt := filepath.Ext(filename)
	extension := strings.ToLower(longExt[1:])
	// check if us audio and save mime type for later
	mime, ok := audioExtensions[extension]
	if !ok {
		return nil
	}
	// set track basics
	track := db.Track{
		Path: fullPath,
	}
	err = tx.Where(track).First(&track).Error // TODO: swap
	if !gorm.IsRecordNotFoundError(err) &&
		!modTime.After(track.UpdatedAt) {
		return nil
	}
	tags, err := readTags(fullPath)
	if err != nil {
		return fmt.Errorf("when reading tags: %v", err)
	}
	trackNumber, totalTracks := tags.Track()
	discNumber, TotalDiscs := tags.Disc()
	track.Path = fullPath
	track.Title = tags.Title()
	track.Artist = tags.Artist()
	track.DiscNumber = uint(discNumber)
	track.TotalDiscs = uint(TotalDiscs)
	track.TotalTracks = uint(totalTracks)
	track.TrackNumber = uint(trackNumber)
	track.Year = uint(tags.Year())
	track.Suffix = extension
	track.ContentType = mime
	track.Size = uint(stat.Size())
	// set album artist {
	albumArtist := db.AlbumArtist{
		Name: tags.AlbumArtist(),
	}
	err = tx.Where(albumArtist).First(&albumArtist).Error
	if gorm.IsRecordNotFoundError(err) {
		albumArtist.Name = tags.AlbumArtist()
		tx.Save(&albumArtist)
	}
	track.AlbumArtistID = albumArtist.ID
	// set album
	album := db.Album{
		AlbumArtistID: albumArtist.ID,
		Title:         tags.Album(),
	}
	err = tx.Where(album).First(&album).Error
	if gorm.IsRecordNotFoundError(err) {
		album.Title = tags.Album()
		album.AlbumArtistID = albumArtist.ID
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
		&db.Album{},
		&db.AlbumArtist{},
		&db.Track{},
		&db.Cover{},
		&db.User{},
	)
	// 🤫🤫🤫
	orm.Exec(`
		INSERT INTO sqlite_sequence(name, seq)
		SELECT 'albums', 500000
		WHERE NOT EXISTS (SELECT * FROM sqlite_sequence)
	`)
	orm.FirstOrCreate(&db.User{}, db.User{
		Name:     "admin",
		Password: "admin",
	})
	orm.FirstOrCreate(&db.User{}, db.User{
		Name:     "senan",
		Password: "password",
	})
	startTime := time.Now()
	tx = orm.Begin()
	err := godirwalk.Walk(os.Args[1], &godirwalk.Options{
		Callback:             handleFile,
		PostChildrenCallback: handleFolderCompletion,
		Unsorted:             true,
	})
	if err != nil {
		log.Fatalf("error when walking: %v\n", err)
	}
	tx.Commit()
	log.Printf("scanned in %s\n", time.Since(startTime))
}
