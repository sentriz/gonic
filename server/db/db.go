package db

import (
	"log"
	"net/url"
	"os"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"gopkg.in/gormigrate.v1"
)

func addMigrationLog(migrs ...gormigrate.Migration) []*gormigrate.Migration {
	log := func(i int, mig gormigrate.MigrateFunc, name string) gormigrate.MigrateFunc {
		return func(tx *gorm.DB) error {
			// print that we're on the ith out of n migrations
			defer log.Printf("migration (%d/%d) '%s' finished", i+1, len(migrs), name)
			return mig(tx)
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

var (
	dbMaxOpenConns = 1
	dbOptions      = url.Values{
		// with this, multiple connections share a single data and schema cache.
		// see https://www.sqlite.org/sharedcache.html
		"cache": {"shared"},
		// with this, the db sleeps for a little while when locked. can prevent
		// a SQLITE_BUSY. see https://www.sqlite.org/c3ref/busy_timeout.html
		"_busy_timeout": {"30000"},
		"_journal_mode": {"WAL"},
		"_foreign_keys": {"true"},
	}
)

type DB struct {
	*gorm.DB
}

func New(path string) (*DB, error) {
	url := url.URL{Path: path}
	url.RawQuery = dbOptions.Encode()
	db, err := gorm.Open("sqlite3", url.String())
	if err != nil {
		return nil, errors.Wrap(err, "with gorm")
	}
	db.SetLogger(log.New(os.Stdout, "gorm ", 0))
	db.DB().SetMaxOpenConns(dbMaxOpenConns)
	migr := gormigrate.New(db, gormigrate.DefaultOptions, addMigrationLog(
		migrationInitSchema,
		migrationCreateInitUser,
		migrationMergePlaylist,
		migrationCreateTranscode,
		migrationAddGenre,
		migrationUpdateTranscodePrefIDX,
		migrationAddAlbumIDX,
	))
	if err = migr.Migrate(); err != nil {
		return nil, errors.Wrap(err, "migrating to latest version")
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

func (db *DB) GetUserFromName(name string) *User {
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

func (db *DB) WithTx(cb func(*gorm.DB)) {
	tx := db.Begin()
	defer tx.Commit()
	cb(tx)
}

type ChunkFunc func(*gorm.DB, []int64) error

func (db *DB) WithTxChunked(data []int64, cb ChunkFunc) error {
	// https://sqlite.org/limits.html
	const size = 999
	tx := db.Begin()
	defer tx.Commit()
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
}
