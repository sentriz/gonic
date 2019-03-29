package model

// Cover represents the covers table
type Cover struct {
	Base
	Album   Album
	AlbumID uint
	Image   []byte
	Path    string `gorm:"not null;unique_index"`
}
