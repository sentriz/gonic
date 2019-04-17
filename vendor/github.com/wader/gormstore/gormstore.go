/*
Package gormstore is a GORM backend for gorilla sessions

Simplest form:

	store := gormstore.New(gorm.Open(...), []byte("secret-hash-key"))

All options:

	store := gormstore.NewOptions(
		gorm.Open(...), // *gorm.DB
		gormstore.Options{
			TableName: "sessions",  // "sessions" is default
			SkipCreateTable: false, // false is default
		},
		[]byte("secret-hash-key"),      // 32 or 64 bytes recommended, required
		[]byte("secret-encyption-key")) // nil, 16, 24 or 32 bytes, optional

		// some more settings, see sessions.Options
		store.SessionOpts.Secure = true
		store.SessionOpts.HttpOnly = true
		store.SessionOpts.MaxAge = 60 * 60 * 24 * 60

If you want periodic cleanup of expired sessions:

		quit := make(chan struct{})
		go store.PeriodicCleanup(1*time.Hour, quit)

For more information about the keys see https://github.com/gorilla/securecookie

For API to use in HTTP handlers see https://github.com/gorilla/sessions
*/
package gormstore

import (
	"encoding/base32"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/context"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
)

const sessionIDLen = 32
const defaultTableName = "sessions"
const defaultMaxAge = 60 * 60 * 24 * 30 // 30 days
const defaultPath = "/"

// Options for gormstore
type Options struct {
	TableName       string
	SkipCreateTable bool
}

// Store represent a gormstore
type Store struct {
	db          *gorm.DB
	opts        Options
	Codecs      []securecookie.Codec
	SessionOpts *sessions.Options
}

type gormSession struct {
	ID        string `sql:"unique_index"`
	Data      string `sql:"type:text"`
	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt time.Time `sql:"index"`

	tableName string `sql:"-"` // just for convenience instead of db.Table(...)
}

// Define a type for context keys so that they can't clash with anything else stored in context
type contextKey string

func (gs *gormSession) TableName() string {
	return gs.tableName
}

// New creates a new gormstore session
func New(db *gorm.DB, keyPairs ...[]byte) *Store {
	return NewOptions(db, Options{}, keyPairs...)
}

// NewOptions creates a new gormstore session with options
func NewOptions(db *gorm.DB, opts Options, keyPairs ...[]byte) *Store {
	st := &Store{
		db:     db,
		opts:   opts,
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		SessionOpts: &sessions.Options{
			Path:   defaultPath,
			MaxAge: defaultMaxAge,
		},
	}
	if st.opts.TableName == "" {
		st.opts.TableName = defaultTableName
	}

	if !st.opts.SkipCreateTable {
		st.db.AutoMigrate(&gormSession{tableName: st.opts.TableName})
	}

	return st
}

// Get returns a session for the given name after adding it to the registry.
func (st *Store) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(st, name)
}

// New creates a session with name without adding it to the registry.
func (st *Store) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(st, name)
	opts := *st.SessionOpts
	session.Options = &opts

	st.MaxAge(st.SessionOpts.MaxAge)

	// try fetch from db if there is a cookie
	if cookie, err := r.Cookie(name); err == nil {
		if err := securecookie.DecodeMulti(name, cookie.Value, &session.ID, st.Codecs...); err != nil {
			return session, nil
		}
		s := &gormSession{tableName: st.opts.TableName}
		if err := st.db.Where("id = ? AND expires_at > ?", session.ID, gorm.NowFunc()).First(s).Error; err != nil {
			return session, nil
		}
		if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values, st.Codecs...); err != nil {
			return session, nil
		}

		context.Set(r, contextKey(name), s)
	}

	return session, nil
}

// Save session and set cookie header
func (st *Store) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	s, _ := context.Get(r, contextKey(session.Name())).(*gormSession)

	// delete if max age is < 0
	if session.Options.MaxAge < 0 {
		if s != nil {
			if err := st.db.Delete(s).Error; err != nil {
				return err
			}
		}
		http.SetCookie(w, sessions.NewCookie(session.Name(), "", session.Options))
		return nil
	}

	data, err := securecookie.EncodeMulti(session.Name(), session.Values, st.Codecs...)
	if err != nil {
		return err
	}
	now := time.Now()
	expire := now.Add(time.Second * time.Duration(session.Options.MaxAge))

	if s == nil {
		// generate random session ID key suitable for storage in the db
		session.ID = strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(sessionIDLen)), "=")
		s = &gormSession{
			ID:        session.ID,
			Data:      data,
			CreatedAt: now,
			UpdatedAt: now,
			ExpiresAt: expire,
			tableName: st.opts.TableName,
		}
		if err := st.db.Create(s).Error; err != nil {
			return err
		}
		context.Set(r, contextKey(session.Name()), s)
	} else {
		s.Data = data
		s.UpdatedAt = now
		s.ExpiresAt = expire
		if err := st.db.Save(s).Error; err != nil {
			return err
		}
	}

	// set session id cookie
	id, err := securecookie.EncodeMulti(session.Name(), session.ID, st.Codecs...)
	if err != nil {
		return err
	}
	http.SetCookie(w, sessions.NewCookie(session.Name(), id, session.Options))

	return nil
}

// MaxAge sets the maximum age for the store and the underlying cookie
// implementation. Individual sessions can be deleted by setting
// Options.MaxAge = -1 for that session.
func (st *Store) MaxAge(age int) {
	st.SessionOpts.MaxAge = age
	for _, codec := range st.Codecs {
		if sc, ok := codec.(*securecookie.SecureCookie); ok {
			sc.MaxAge(age)
		}
	}
}

// MaxLength restricts the maximum length of new sessions to l.
// If l is 0 there is no limit to the size of a session, use with caution.
// The default is 4096 (default for securecookie)
func (st *Store) MaxLength(l int) {
	for _, c := range st.Codecs {
		if codec, ok := c.(*securecookie.SecureCookie); ok {
			codec.MaxLength(l)
		}
	}
}

// Cleanup deletes expired sessions
func (st *Store) Cleanup() {
	st.db.Delete(&gormSession{tableName: st.opts.TableName}, "expires_at <= ?", gorm.NowFunc())
}

// PeriodicCleanup runs Cleanup every interval. Close quit channel to stop.
func (st *Store) PeriodicCleanup(interval time.Duration, quit <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			st.Cleanup()
		case <-quit:
			return
		}
	}
}
