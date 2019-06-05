package model

import (
	"path"
	"time"

	"github.com/sentriz/gonic/mime"
)

type Artist struct {
	IDBase
	Name   string  `gorm:"not null; unique_index"`
	Albums []Album `gorm:"foreignkey:TagArtistID"`
}

type Track struct {
	IDBase
	CrudBase
	Album          Album
	AlbumID        int    `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Filename       string `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null"`
	Artist         Artist
	ArtistID       int    `gorm:"not null; index" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
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

func (t *Track) Ext() string {
	longExt := path.Ext(t.Filename)
	if len(longExt) < 1 {
		return ""
	}
	return longExt[1:]
}

func (t *Track) MIME() string {
	ext := t.Ext()
	return mime.Types[ext]
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
	User    User
	UserID  int `gorm:"not null; index" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Album   Album
	AlbumID int       `gorm:"not null; index" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Time    time.Time `sql:"default: null"`
	Count   int
}

type Album struct {
	IDBase
	CrudBase
	LeftPath    string `gorm:"unique_index:idx_left_path_right_path"`
	RightPath   string `gorm:"not null; unique_index:idx_left_path_right_path" sql:"default: null"`
	Parent      *Album
	ParentID    int    `sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Cover       string `sql:"default: null"`
	TagArtist   Artist
	TagArtistID int    `gorm:"index" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	TagTitle    string `gorm:"index" sql:"default: null"`
	TagYear     int    `sql:"default: null"`
	Tracks      []Track
	IsNew       bool `gorm:"-"`
}
