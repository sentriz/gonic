package db

import (
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"senan.xyz/g/gonic/model"
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

type DB struct {
	*gorm.DB
}

func New(path string) (*DB, error) {
	pathAndArgs := fmt.Sprintf("%s?%s", path, dbOptions.Encode())
	db, err := gorm.Open("sqlite3", pathAndArgs)
	if err != nil {
		return nil, errors.Wrap(err, "with gorm")
	}
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	db.DB().SetMaxOpenConns(dbMaxOpenConns)
	db.AutoMigrate(
		model.Artist{},
		model.Track{},
		model.User{},
		model.Setting{},
		model.Play{},
		model.Album{},
	)
	db.FirstOrCreate(&model.User{}, model.User{
		Name:     "admin",
		Password: "admin",
		IsAdmin:  true,
	})
	return &DB{DB: db}, nil
}

func NewMock() (*DB, error) {
	return New(":memory:")
}

func (db *DB) GetSetting(key string) string {
	setting := &model.Setting{}
	db.
		Where("key = ?", key).
		First(setting)
	return setting.Value
}

func (db *DB) SetSetting(key, value string) {
	db.
		Where(model.Setting{Key: key}).
		Assign(model.Setting{Value: value}).
		FirstOrCreate(&model.Setting{})
}

func (db *DB) GetUserFromName(name string) *model.User {
	user := &model.User{}
	err := db.
		Where("name = ?", name).
		First(user).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	return user
}

func (db *DB) WithTx(cb func(tx *gorm.DB)) {
	tx := db.Begin()
	defer tx.Commit()
	cb(tx)
}
