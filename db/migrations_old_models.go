package db

import "time"

type __OldPlaylist struct { //nolint: staticcheck
	ID         int `gorm:"primary_key"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	User       *User
	UserID     int `sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Name       string
	Comment    string
	TrackCount int
	Items      string
	IsPublic   bool `sql:"default: null"`
}

func (__OldPlaylist) TableName() string {
	return "playlists"
}
