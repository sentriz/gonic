//nolint:lll
package model

import (
	"path"
	"time"

	"senan.xyz/g/gonic/mime"
)

type Artist struct {
	ID         int      `gorm:"primary_key"`
	Name       string   `gorm:"not null; unique_index"`
	NameUDec   string   `sql:"default: null"`
	Albums     []*Album `gorm:"foreignkey:TagArtistID"`
	AlbumCount int      `sql:"-"`
}

func (a *Artist) IndexName() string {
	if len(a.NameUDec) > 0 {
		return a.NameUDec
	}
	return a.Name
}

type Track struct {
	ID             int `gorm:"primary_key"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Filename       string `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null"`
	FilenameUDec   string `sql:"default: null"`
	Album          *Album
	AlbumID        int `gorm:"not null; unique_index:idx_folder_filename" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Artist         *Artist
	ArtistID       int    `gorm:"not null" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	Size           int    `gorm:"not null" sql:"default: null"`
	Length         int    `sql:"default: null"`
	Bitrate        int    `sql:"default: null"`
	TagTitle       string `sql:"default: null"`
	TagTitleUDec   string `sql:"default: null"`
	TagTrackArtist string `sql:"default: null"`
	TagTrackNumber int    `sql:"default: null"`
	TagDiscNumber  int    `sql:"default: null"`
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
	ID            int `gorm:"primary_key"`
	CreatedAt     time.Time
	Name          string `gorm:"not null; unique_index" sql:"default: null"`
	Password      string `gorm:"not null" sql:"default: null"`
	LastFMSession string `sql:"default: null"`
	IsAdmin       bool   `sql:"default: null"`
}

type Setting struct {
	Key   string `gorm:"not null; primary_key; auto_increment:false" sql:"default: null"`
	Value string `sql:"default: null"`
}

type Play struct {
	ID      int `gorm:"primary_key"`
	User    *User
	UserID  int `gorm:"not null; index" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Album   *Album
	AlbumID int       `gorm:"not null; index" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Time    time.Time `sql:"default: null"`
	Count   int
}

type Album struct {
	ID            int `gorm:"primary_key"`
	UpdatedAt     time.Time
	ModifiedAt    time.Time
	LeftPath      string `gorm:"unique_index:idx_left_path_right_path"`
	RightPath     string `gorm:"not null; unique_index:idx_left_path_right_path" sql:"default: null"`
	RightPathUDec string `sql:"default: null"`
	Parent        *Album
	ParentID      int    `sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Cover         string `sql:"default: null"`
	TagArtist     *Artist
	TagArtistID   int    `sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	TagTitle      string `sql:"default: null"`
	TagTitleUDec  string `sql:"default: null"`
	TagYear       int    `sql:"default: null"`
	Tracks        []*Track
	ChildCount    int  `sql:"-"`
	ReceivedPaths bool `gorm:"-"`
	ReceivedTags  bool `gorm:"-"`
}

func (a *Album) IndexRightPath() string {
	if len(a.RightPathUDec) > 0 {
		return a.RightPathUDec
	}
	return a.RightPath
}
