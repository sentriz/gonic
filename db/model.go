package db

// Album represents the albums table
type Album struct {
	Base
	AlbumArtist   AlbumArtist
	AlbumArtistID uint
	Title         string `gorm:"not null;index"`
	Tracks        []Track
}

// AlbumArtist represents the AlbumArtists table
type AlbumArtist struct {
	Base
	Albums []Album
	Name   string `gorm:"not null;unique_index"`
}

// Track represents the tracks table
type Track struct {
	Base
	Album         Album
	AlbumID       uint
	AlbumArtist   AlbumArtist
	AlbumArtistID uint
	Artist        string
	Bitrate       uint
	Codec         string
	DiscNumber    uint
	Duration      uint
	Title         string
	TotalDiscs    uint
	TotalTracks   uint
	TrackNumber   uint
	Year          uint
	Suffix        string
	ContentType   string
	Size          uint
	Path          string `gorm:"not null;unique_index"`
}

// Cover represents the covers table
type Cover struct {
	CrudBase
	AlbumID uint `gorm:"primary_key;auto_increment:false"`
	Album   Album
	Image   []byte
	Path    string `gorm:"not null;unique_index"`
}

// User represents the users table
type User struct {
	IDBase
	Username string `gorm:"not null;unique_index"`
	Password string
}
