package model

// Cover represents the covers table
type Cover struct {
	Base
	Album   Album
	AlbumID string
	Image   []byte
	Path    string
}
