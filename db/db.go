package db

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/jinzhu/gorm"
)

func DefaultOptions() url.Values {
	return url.Values{
		// with this, multiple connections share a single data and schema cache.
		// see https://www.sqlite.org/sharedcache.html
		"cache": {"shared"},
		// with this, the db sleeps for a little while when locked. can prevent
		// a SQLITE_BUSY. see https://www.sqlite.org/c3ref/busy_timeout.html
		"_busy_timeout": {"30000"},
		"_journal_mode": {"WAL"},
		"_foreign_keys": {"true"},
	}
}

func mockOptions() url.Values {
	return url.Values{
		"_foreign_keys": {"true"},
	}
}

type DB struct {
	*gorm.DB
}

func New(path string, options url.Values) (*DB, error) {
	// https://github.com/mattn/go-sqlite3#connection-string
	url := url.URL{
		Scheme: "file",
		Opaque: path,
	}
	url.RawQuery = options.Encode()
	db, err := gorm.Open("sqlite3", url.String())
	if err != nil {
		return nil, fmt.Errorf("with gorm: %w", err)
	}
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	db.DB().SetMaxOpenConns(1)
	return &DB{DB: db}, nil
}

func NewMock() (*DB, error) {
	return New(":memory:", mockOptions())
}

func (db *DB) GetSetting(key string) (string, error) {
	var setting Setting
	if err := db.Where("key=?", key).First(&setting).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	return setting.Value, nil
}

func (db *DB) SetSetting(key, value string) error {
	return db.
		Where("key=?", key).
		Assign(Setting{Key: key, Value: value}).
		FirstOrCreate(&Setting{}).
		Error
}

func (db *DB) InsertBulkLeftMany(table string, head []string, left int, col []int) error {
	if len(col) == 0 {
		return nil
	}
	var rows []string
	var values []interface{}
	for _, c := range col {
		rows = append(rows, "(?, ?)")
		values = append(values, left, c)
	}
	q := fmt.Sprintf("INSERT OR IGNORE INTO %q (%s) VALUES %s",
		table,
		strings.Join(head, ", "),
		strings.Join(rows, ", "),
	)
	return db.Exec(q, values...).Error
}

func (db *DB) GetUserByID(id int) *User {
	var user User
	err := db.
		Where("id=?", id).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return &user
}

func (db *DB) GetUserByName(name string) *User {
	var user User
	err := db.
		Where("name=?", name).
		First(&user).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	return &user
}

func (db *DB) Begin() *DB {
	return &DB{DB: db.DB.Begin()}
}

type ChunkFunc func(*gorm.DB, []int64) error

func (db *DB) TransactionChunked(data []int64, cb ChunkFunc) error {
	if len(data) == 0 {
		return nil
	}
	// https://sqlite.org/limits.html
	const size = 999
	return db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < len(data); i += size {
			end := i + size
			if end > len(data) {
				end = len(data)
			}
			if err := cb(tx, data[i:end]); err != nil {
				return err
			}
		}
		return nil
	})
}
