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
	ID uint `gorm:"primary_key"`
}

// Base is the base model with an auto incrementing primary key
type Base struct {
	IDBase
	CrudBase
}
