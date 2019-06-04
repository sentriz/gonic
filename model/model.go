package model

import (
	"time"
)

// q:  what in tarnation are the `IsNew`s for?
// a:  it's a bit of a hack - but we set a models IsNew to true if
//     we just filled it in for the first time, so when it comes
//     time to insert them (post children callback) we can check for
//     that bool being true - since it won't be true if it was already
//     in the db

type Artist struct {
	IDBase
	CrudBase
	Name    string `gorm:"not null; unique_index"`
	Folders []Folder
}

type Track struct {
	IDBase
	CrudBase
	Folder         Folder
	FolderID       int    `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null; type:int REFERENCES folders(id) ON DELETE CASCADE"`
	Filename       string `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null"`
	Artist         Artist
	ArtistID       int    `gorm:"not null; index" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	ContentType    string `gorm:"not null" sql:"default: null"`
	Duration       int    `gorm:"not null" sql:"default: null"`
	Size           int    `gorm:"not null" sql:"default: null"`
	Bitrate        int    `gorm:"not null" sql:"default: null"`
	TagDiscNumber  int    `sql:"default: null"`
	TagTitle       string `sql:"default: null"`
	TagTotalDiscs  int    `sql:"default: null"`
	TagTotalTracks int    `sql:"default: null"`
	TagTrackArtist string `sql:"default: null"`
	TagTrackNumber int    `sql:"default: null"`
	TagYear        int    `sql:"default: null"`
}

type User struct {
	IDBase
	CrudBase
	Name          string `gorm:"not null; unique_index" sql:"default: null"`
	Password      string `gorm:"not null" sql:"default: null"`
	LastFMSession string `sql:"default: null"`
	IsAdmin       bool   `sql:"default: null"`
}

type Setting struct {
	CrudBase
	Key   string `gorm:"not null; primary_key; auto_increment:false" sql:"default: null"`
	Value string `sql:"default: null"`
}

type Play struct {
	IDBase
	User     User
	UserID   int `gorm:"not null; index" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Folder   Folder
	FolderID int       `gorm:"not null; index" sql:"default: null; type:int REFERENCES folders(id) ON DELETE CASCADE"`
	Time     time.Time `sql:"default: null"`
	Count    int
}

type Folder struct {
	IDBase
	CrudBase
	Path          string `gorm:"not null; unique_index" sql:"default: null"`
	Parent        *Folder
	ParentID      int `sql:"default: null; type:int REFERENCES folders(id) ON DELETE CASCADE"`
	AlbumArtist   Artist
	AlbumArtistID int    `gorm:"index" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	AlbumTitle    string `gorm:"index" sql:"default: null"`
	AlbumYear     int    `sql:"default: null"`
	Cover         string `sql:"default: null"`
	Tracks        []Track
	IsNew         bool `gorm:"-"`
}
