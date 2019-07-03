package db

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
)

var (
	dbMaxOpenConns = 1
	dbOptions      = url.Values{
		// with this, multiple connections share a single data and schema cache.
		// see https://www.sqlite.org/sharedcache.html
		"cache": []string{"shared"},
		// with this, the db sleeps for a little while when locked. can prevent
		// a SQLITE_BUSY. see https://www.sqlite.org/c3ref/busy_timeout.html
		"_busy_timeout": []string{"30000"},
	}
)

func New(path string) (*gorm.DB, error) {
	pathAndArgs := fmt.Sprintf("%s?%s", path, dbOptions.Encode())
	db, err := gorm.Open("sqlite3", pathAndArgs)
	if err != nil {
		return nil, errors.Wrap(err, "with gorm")
	}
	db.DB().SetMaxOpenConns(dbMaxOpenConns)
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	return db, nil
}
