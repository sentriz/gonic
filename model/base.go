package model

import (
	"time"
)

type CrudBase struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

type IDBase struct {
	ID *int `gorm:"primary_key"`
}
