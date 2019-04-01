package db

import (
	"time"

	"github.com/jinzhu/gorm"
	"github.com/twinj/uuid"
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

// BaseWithUUID is the base model with an UUIDv4 primary key
type BaseWithUUID struct {
	IDBase
	CrudBase
}

// BeforeCreate is called by GORM to set the UUID primary key
func (b *BaseWithUUID) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("ID", uuid.NewV4().String())
}
