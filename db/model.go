package db

import "time"

// Album represents the albums table
type Album struct {
	IDBase
	CrudBase
	AlbumArtist   AlbumArtist
	AlbumArtistID int    `gorm:"index"`
	Title         string `gorm:"not null;index"`
	// an Album having a `Path` is a little weird when browsing by tags
	// (for the most part - the library's folder structure is treated as
	// if it were flat), but this solves the "American Football problem"
	// https://en.wikipedia.org/wiki/American_Football_(band)#Discography
	Path    string `gorm:"not null;unique_index"`
	CoverID int
	Cover   Cover
	Tracks  []Track
}

// AlbumArtist represents the AlbumArtists table
type AlbumArtist struct {
	IDBase
	CrudBase
	Albums []Album
	Name   string `gorm:"not null;unique_index"`
}

// Track represents the tracks table
type Track struct {
	IDBase
	CrudBase
	Album         Album
	AlbumID       int `gorm:"index"`
	AlbumArtist   AlbumArtist
	AlbumArtistID int
	Artist        string
	Bitrate       int
	Codec         string
	DiscNumber    int
	Duration      int
	Title         string
	TotalDiscs    int
	TotalTracks   int
	TrackNumber   int
	Year          int
	Suffix        string
	ContentType   string
	Size          int
	FolderID      int
	Path          string `gorm:"not null;unique_index"`
}

// Cover represents the covers table
type Cover struct {
	IDBase
	CrudBase
	Image []byte
	Path  string `gorm:"not null;unique_index"`
}

// User represents the users table
type User struct {
	IDBase
	CrudBase
	Name          string `gorm:"not null;unique_index"`
	Password      string
	LastFMSession string
	IsAdmin       bool
}

// Setting represents the settings table
type Setting struct {
	CrudBase
	Key   string `gorm:"primary_key;auto_increment:false"`
	Value string
}

// Play represents the settings table
type Play struct {
	IDBase
	User    User
	UserID  int
	Track   Track
	TrackID int
	Time    time.Time
}

// Folder represents the settings table
type Folder struct {
	IDBase
	CrudBase
	Name     string
	Path     string  `gorm:"not null;unique_index"`
	Parent   *Folder `gorm:"foreignkey:ParentID"`
	ParentID int
	CoverID  int
	Cover    Cover
}
