package db

// Album represents the albums table
type Album struct {
	Base
	Artist   Artist
	ArtistID uint
	Title    string `gorm:"not null;index"`
	Tracks   []Track
}

// Artist represents the artists table
type Artist struct {
	Base
	Albums []Album
	Name   string `gorm:"not null;unique_index"`
}

// Track represents the tracks table
type Track struct {
	Base
	Album       Album
	AlbumID     uint
	Artist      Artist
	ArtistID    uint
	Bitrate     uint
	Codec       string
	DiscNumber  uint
	Duration    uint
	Title       string
	TotalDiscs  uint
	TotalTracks uint
	TrackNumber uint
	Year        uint
	Suffix      string
	ContentType string
	Path        string `gorm:"not null;unique_index"`
}

// Cover represents the covers table
type Cover struct {
	Base
	Album   Album
	AlbumID uint
	Image   []byte
	Path    string `gorm:"not null;unique_index"`
}

// User represents the users table
type User struct {
	IDBase
	Username string `gorm:"not null;unique_index"`
	Password string
}
