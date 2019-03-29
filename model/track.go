package model

// Track represents the tracks table
type Track struct {
	Base
	Album       Album
	AlbumID     uint
	Artist      Artist
	ArtistID    uint
	Bitrate     int
	Codec       string
	DiscNumber  int
	Duration    int
	Title       string
	TotalDiscs  int
	TotalTracks int
	TrackNumber int
	Year        int
	Path        string `gorm:"not null;unique_index"`
}
