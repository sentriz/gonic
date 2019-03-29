package model

// Album represents the albums table
type Album struct {
	Base
	Artist   Artist
	ArtistID uint
	Title    string `gorm:"not null;index"`
	Tracks   []Track
}
