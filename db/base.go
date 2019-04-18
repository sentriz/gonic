package db

import (
	"time"
)

type CrudBase struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

type IDBase struct {
	ID int `gorm:"primary_key"`
}
