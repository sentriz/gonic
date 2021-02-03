package db

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/securecookie"
	"github.com/jinzhu/gorm"

	"gopkg.in/gormigrate.v1"
)

// wrapMigrations wraps a list of migrations to add logging and transactions
func wrapMigrations(migrs ...gormigrate.Migration) []*gormigrate.Migration {
	log := func(i int, mig gormigrate.MigrateFunc, name string) gormigrate.MigrateFunc {
		return func(db *gorm.DB) error {
			// print that we're on the ith out of n migrations
			defer log.Printf("migration (%d/%d) '%s' finished", i+1, len(migrs), name)
			return db.Transaction(mig)
		}
	}
	ret := make([]*gormigrate.Migration, 0, len(migrs))
	for i, mig := range migrs {
		ret = append(ret, &gormigrate.Migration{
			ID:       mig.ID,
			Rollback: mig.Rollback,
			Migrate:  log(i, mig.Migrate, mig.ID),
		})
	}
	return ret
}

func defaultOptions() url.Values {
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

type DB struct {
	*gorm.DB
}

func New(path string) (*DB, error) {
	// https://github.com/mattn/go-sqlite3#connection-string
	url := url.URL{
		Scheme: "file",
		Opaque: path,
	}
	url.RawQuery = defaultOptions().Encode()
	db, err := gorm.Open("sqlite3", url.String())
	if err != nil {
		return nil, fmt.Errorf("with gorm: %w", err)
	}
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	db.DB().SetMaxOpenConns(1)
	migrOptions := &gormigrate.Options{
		TableName:      "migrations",
		IDColumnName:   "id",
		IDColumnSize:   255,
		UseTransaction: false,
	}
	migr := gormigrate.New(db, migrOptions, wrapMigrations(
		migrateInitSchema(),
		migrateCreateInitUser(),
		migrateMergePlaylist(),
		migrateCreateTranscode(),
		migrateAddGenre(),
		migrateUpdateTranscodePrefIDX(),
		migrateAddAlbumIDX(),
		migrateMultiGenre(),
		migrateListenBrainz(),
		migratePodcast(),
	))
	if err = migr.Migrate(); err != nil {
		return nil, fmt.Errorf("migrating to latest version: %w", err)
	}
	return &DB{DB: db}, nil
}

func NewMock() (*DB, error) {
	return New(":memory:")
}

func (db *DB) GetSetting(key string) string {
	setting := &Setting{}
	db.
		Where("key=?", key).
		First(setting)
	return setting.Value
}

func (db *DB) SetSetting(key, value string) {
	db.
		Where(Setting{Key: key}).
		Assign(Setting{Value: value}).
		FirstOrCreate(&Setting{})
}

func (db *DB) GetOrCreateKey(key string) string {
	value := db.GetSetting(key)
	if value == "" {
		value = string(securecookie.GenerateRandomKey(32))
		db.SetSetting(key, value)
	}
	return value
}

func (db *DB) InsertBulkLeftMany(table string, head []string, left int, col []int) error {
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
	user := &User{}
	err := db.
		Where("id=?", id).
		First(user).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	return user
}

func (db *DB) GetUserByName(name string) *User {
	user := &User{}
	err := db.
		Where("name=?", name).
		First(user).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return nil
	}
	return user
}

func (db *DB) Begin() *DB {
	return &DB{DB: db.DB.Begin()}
}

type ChunkFunc func(*gorm.DB, []int64) error

func (db *DB) TransactionChunked(data []int64, cb ChunkFunc) error {
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
