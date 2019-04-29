package db

import (
	"log"
	"path/filepath"
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

// New creates a new GORM connection to the mock database
func NewMock() *gorm.DB {
	// projectRoot presumes this file is `<root>/db/db.go`
	dbPath, _ := filepath.Abs(filepath.Join(cFile, "../../test_data/mock.db"))
	db, err := gorm.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("when opening mock database: %v\n", err)
	}
	return db
}
