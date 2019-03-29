package model

// Artist represents the artists table
type Artist struct {
	Base
	Albums []Album
	Name   string `gorm:"not null;unique_index"`
}
