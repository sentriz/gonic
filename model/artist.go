package model

// Artist represents the artists table
type Artist struct {
	BaseWithUUID
	Albums []Album
	Name   string `gorm:"unique;n"`
}
