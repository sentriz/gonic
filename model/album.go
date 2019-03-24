package model

// Album represents the albums table
type Album struct {
	BaseWithUUID
	Artist   Artist
	ArtistID string
	Title    string
	Tracks   []Track
}
