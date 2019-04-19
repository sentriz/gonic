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
	seenTracks = make(map[string]bool)
)

type lastAlbum struct {
	coverModTime time.Time // 1st needed for cover insertion
	coverPath    string    // 2rd needed for cover insertion
	id           int       // 3nd needed for cover insertion
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
	// skip if the record exists and hasn't been modified since
	// the last scan
	err := tx.Where(cover).First(&cover).Error
	if !gorm.IsRecordNotFoundError(err) &&
		cLastAlbum.coverModTime.Before(cover.UpdatedAt) {
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
	// add the full path to the seen set. later used to delete
	// tracks that are no longer on filesystem and still in the
	// database
	seenTracks[fullPath] = true
	// set track basics
	track := db.Track{
		Path: fullPath,
	}
	// skip if the record exists and hasn't been modified since
	// the last scan
	err = tx.Where(track).First(&track).Error
	if !gorm.IsRecordNotFoundError(err) &&
		modTime.Before(track.UpdatedAt) {
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
	track.Suffix = extension
	track.ContentType = mime
	track.Size = int(stat.Size())
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

func createDatabase() {
	tx.AutoMigrate(
		&db.Album{},
		&db.AlbumArtist{},
		&db.Track{},
		&db.Cover{},
		&db.User{},
		&db.Setting{},
		&db.Play{},
	)
	// set starting value for `albums` table's
	// auto increment
	tx.Exec(`
        INSERT INTO sqlite_sequence(name, seq)
        SELECT 'albums', 500000
        WHERE  NOT EXISTS (SELECT *
                           FROM   sqlite_sequence);
	`)
	// create the first user if there is none
	tx.FirstOrCreate(&db.User{}, db.User{
		Name:     "admin",
		Password: "admin",
		IsAdmin:  true,
	})
}

func cleanDatabase() {
	// delete tracks not on filesystem
	var tracks []*db.Track
	tx.Select("id, path").Find(&tracks)
	for _, track := range tracks {
		_, ok := seenTracks[track.Path]
		if ok {
			continue
		}
		tx.Delete(&track)
		log.Println("removed", track.Path)
	}
	// delete albums without tracks
	tx.Exec(`
        DELETE FROM albums
        WHERE  (SELECT count(id)
                FROM   tracks
                WHERE  album_id = albums.id) = 0;
	`)
	// delete artists without tracks
	tx.Exec(`
        DELETE FROM album_artists
        WHERE  (SELECT count(id)
                FROM   albums
                WHERE  album_artist_id = album_artists.id) = 0;
	`)
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
		Callback:             handleFile,
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
