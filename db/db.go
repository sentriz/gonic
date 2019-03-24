package db

import (
	"log"

	"github.com/jinzhu/gorm"
)

// New creates a new GORM connection to the database
func New() *gorm.DB {
	db, err := gorm.Open("sqlite3", "gonic.db")
	if err != nil {
		log.Printf("when opening database: %v\n", err)
	}
	return db
}
