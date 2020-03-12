//nolint:lll
package db

import (
	"path"
	"strconv"
	"strings"
	"time"

	"senan.xyz/g/gonic/mime"
)

func splitInt(in, sep string) []int {
	if len(in) == 0 {
		return []int{}
	}
	parts := strings.Split(in, sep)
	ret := make([]int, 0, len(parts))
	for _, p := range parts {
		i, _ := strconv.Atoi(p)
		ret = append(ret, i)
	}
	return ret
}

func joinInt(in []int, sep string) string {
	if in == nil {
		return ""
	}
	strs := make([]string, 0, len(in))
	for _, i := range in {
		strs = append(strs, strconv.Itoa(i))
	}
	return strings.Join(strs, sep)
}

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
	TagBrainzID    string `sql:"default: null"`
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

func (t *Track) RelPath() string {
	if t.Album == nil {
		return ""
	}
	return path.Join(
		t.Album.LeftPath,
		t.Album.RightPath,
		t.Filename,
	)
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
	TagBrainzID   string `sql:"default: null"`
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

type Playlist struct {
	ID         int `gorm:"primary_key"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	User       *User
	UserID     int `sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Name       string
	Comment    string
	TrackCount int
	Items      string
}

func (p *Playlist) GetItems() []int {
	return splitInt(p.Items, ",")
}

func (p *Playlist) SetItems(items []int) {
	p.Items = joinInt(items, ",")
	p.TrackCount = len(items)
}

type PlayQueue struct {
	ID        int `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time
	User      *User
	UserID    int `sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Current   int
	Position  int
	ChangedBy string
	Items     string
}

func (p *PlayQueue) GetItems() []int {
	return splitInt(p.Items, ",")
}

func (p *PlayQueue) SetItems(items []int) {
	p.Items = joinInt(items, ",")
}

type TranscodePreference struct {
	User    *User
	UserID  int    `sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Client  string `gorm:"not null; unique_index:idx_client_profile" sql:"default: null"`
	Profile string `gorm:"not null; unique_index:idx_client_profile" sql:"default: null"`
}
