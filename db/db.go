package db

import (
	"log"
	"runtime"

	"github.com/jinzhu/gorm"
)

var (
	// cFile is the path to this go file
	_, cFile, _, _ = runtime.Caller(0)
)

// New creates a new GORM connection to the database
func New() *gorm.DB {
	db, err := gorm.Open("sqlite3", "gonic.db")
	if err != nil {
		log.Printf("when opening database: %v\n", err)
	}
	return db
}
