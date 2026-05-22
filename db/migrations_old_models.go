// nolint: staticcheck
package db

import "time"

type __OldPlay struct {
	ID      int `gorm:"primary_key"`
	User    *User
	UserID  int `gorm:"not null; index" sql:"default: null; type:int REFERENCES users(id) ON DELETE CASCADE"`
	Album   *Album
	AlbumID int       `gorm:"not null; index" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	Time    time.Time `sql:"default: null"`
	Count   int
	Length  int
}

func (__OldPlay) TableName() string { return "plays" }

type __OldPlaylist struct {
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

func (__OldPlaylist) TableName() string { return "playlists" }

type __OldAlbumArtist struct {
	AlbumID    int    `gorm:"not null; unique_index:idx_album_id_artist_id" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
	ArtistID   int    `gorm:"not null; unique_index:idx_album_id_artist_id" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	CreditedAs string `sql:"default: null"`
}

func (__OldAlbumArtist) TableName() string { return "album_artists" }

type __OldTrackArtist struct {
	TrackID    int    `gorm:"not null; unique_index:idx_track_id_artist_id" sql:"default: null; type:int REFERENCES tracks(id) ON DELETE CASCADE"`
	ArtistID   int    `gorm:"not null; unique_index:idx_track_id_artist_id" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	CreditedAs string `sql:"default: null"`
}

func (__OldTrackArtist) TableName() string { return "track_artists" }

type __OldTrackContributor struct {
	TrackID    int    `gorm:"not null; unique_index:idx_track_contributor" sql:"default: null; type:int REFERENCES tracks(id) ON DELETE CASCADE"`
	ArtistID   int    `gorm:"not null; unique_index:idx_track_contributor" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	Role       string `gorm:"not null; unique_index:idx_track_contributor" sql:"default: null"`
	CreditedAs string `sql:"default: null"`
}

func (__OldTrackContributor) TableName() string { return "track_contributors" }

type __OldArtistAppearances struct {
	ArtistID int `gorm:"not null; unique_index:idx_artist_id_album_id" sql:"default: null; type:int REFERENCES artists(id) ON DELETE CASCADE"`
	AlbumID  int `gorm:"not null; unique_index:idx_artist_id_album_id" sql:"default: null; type:int REFERENCES albums(id) ON DELETE CASCADE"`
}

func (__OldArtistAppearances) TableName() string { return "artist_appearances" }
