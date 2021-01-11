// Package ctrladmin provides HTTP handlers for admin UI
package ctrladmin

import (
	"encoding/gob"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/sessions"
	"github.com/oxtoacart/bpool"
	"github.com/wader/gormstore"

	"go.senan.xyz/gonic/server/assets"
	"go.senan.xyz/gonic/server/ctrlbase"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/podcasts"
	"go.senan.xyz/gonic/version"
)

type CtxKey int

const (
	CtxUser CtxKey = iota
	CtxSession
)

// extendFromPaths /extends/ the given template for every asset
// with given prefix
func extendFromPaths(b *template.Template, p string) *template.Template {
	assets.PrefixDo(p, func(_ string, asset *assets.EmbeddedAsset) {
		tmplStr := string(asset.Bytes)
		b = template.Must(b.Parse(tmplStr))
	})
	return b
}

// extendFromPaths /clones/ the given template for every asset
// with given prefix, extends it, and insert it into a new map
func pagesFromPaths(b *template.Template, p string) map[string]*template.Template {
	ret := map[string]*template.Template{}
	assets.PrefixDo(p, func(path string, asset *assets.EmbeddedAsset) {
		tmplKey := filepath.Base(path)
		clone := template.Must(b.Clone())
		tmplStr := string(asset.Bytes)
		ret[tmplKey] = template.Must(clone.Parse(tmplStr))
	})
	return ret
}

const (
	prefixPartials = "partials"
	prefixLayouts  = "layouts"
	prefixPages    = "pages"
)

func funcMap() template.FuncMap {
	return template.FuncMap{
		"noCache": func(in string) string {
			parsed, _ := url.Parse(in)
			params := parsed.Query()
			params.Set("v", version.VERSION)
			parsed.RawQuery = params.Encode()
			return parsed.String()
		},
		"date": func(in time.Time) string {
			return strings.ToLower(in.Format("Jan 02, 2006"))
		},
		"dateHuman": humanize.Time,
	}
}

type Controller struct {
	*ctrlbase.Controller
	buffPool  *bpool.BufferPool
	templates map[string]*template.Template
	sessDB    *gormstore.Store
	Podcasts  *podcasts.Podcasts
}

func New(b *ctrlbase.Controller, sessDB *gormstore.Store, podcasts *podcasts.Podcasts) *Controller {
	tmplBase := template.
		New("layout").
		Funcs(sprig.FuncMap()).
		Funcs(funcMap()).       // static
		Funcs(template.FuncMap{ // from base
			"path": b.Path,
		})
	tmplBase = extendFromPaths(tmplBase, prefixPartials)
	tmplBase = extendFromPaths(tmplBase, prefixLayouts)
	return &Controller{
		Controller: b,
		buffPool:   bpool.NewBufferPool(64),
		templates:  pagesFromPaths(tmplBase, prefixPages),
		sessDB:     sessDB,
		Podcasts:   podcasts,
	}
}

type templateData struct {
	// common
	Flashes []interface{}
	User    *db.User
	Version string
	// home
	AlbumCount           int
	ArtistCount          int
	TrackCount           int
	RequestRoot          string
	RecentFolders        []*db.Album
	AllUsers             []*db.User
	LastScanTime         time.Time
	IsScanning           bool
	Playlists            []*db.Playlist
	TranscodePreferences []*db.TranscodePreference
	TranscodeProfiles    []string
	//
	CurrentLastFMAPIKey    string
	CurrentLastFMAPISecret string
	DefaultListenBrainzURL string
	SelectedUser           *db.User
	//
	Podcasts        []*db.Podcast
}

type Response struct {
	// code is 200
	template string
	data     *templateData
	// code is 303
	redirect string
	flashN   []string // normal
	flashW   []string // warning
	// code is >= 400
	code int
	err  string
}

type (
	handlerAdmin    func(r *http.Request) *Response
	handlerAdminRaw func(w http.ResponseWriter, r *http.Request)
)

func (c *Controller) H(h handlerAdmin) http.Handler {
	// TODO: break this up a bit
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := h(r)
		session, ok := r.Context().Value(CtxSession).(*sessions.Session)
		if ok {
			sessAddFlashN(session, resp.flashN)
			sessAddFlashW(session, resp.flashW)
			if err := session.Save(r, w); err != nil {
				http.Error(w, fmt.Sprintf("error saving session: %v", err), 500)
				return
			}
		}
		if resp.redirect != "" {
			to := resp.redirect
			if strings.HasPrefix(to, "/") {
				to = c.Path(to)
			}
			http.Redirect(w, r, to, http.StatusSeeOther)
			return
		}
		if resp.err != "" {
			http.Error(w, resp.err, resp.code)
			return
		}
		if resp.template == "" {
			http.Error(w, "useless handler return", 500)
			return
		}
		if resp.data == nil {
			resp.data = &templateData{}
		}
		resp.data.Version = version.VERSION
		if session != nil {
			resp.data.Flashes = session.Flashes()
			if err := session.Save(r, w); err != nil {
				http.Error(w, fmt.Sprintf("error saving session: %v", err), 500)
				return
			}
		}
		if user, ok := r.Context().Value(CtxUser).(*db.User); ok {
			resp.data.User = user
		}
		buff := c.buffPool.Get()
		defer c.buffPool.Put(buff)
		tmpl, ok := c.templates[resp.template]
		if !ok {
			http.Error(w, fmt.Sprintf("finding template %q", resp.template), 500)
			return
		}
		if err := tmpl.Execute(buff, resp.data); err != nil {
			http.Error(w, fmt.Sprintf("executing template: %v", err), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if resp.code != 0 {
			w.WriteHeader(resp.code)
		}
		if _, err := buff.WriteTo(w); err != nil {
			log.Printf("error writing to response buffer: %v\n", err)
		}
	})
}

func (c *Controller) HR(h handlerAdminRaw) http.Handler {
	return http.HandlerFunc(h)
}

// ## begin utilities
// ## begin utilities
// ## begin utilities

type FlashType string

const (
	FlashNormal  = FlashType("normal")
	FlashWarning = FlashType("warning")
)

type Flash struct {
	Message string
	Type    FlashType
}

//nolint:gochecknoinits // for now I think it's nice that our types and their
// gob registrations are next to each other, in case there's more added later)
func init() {
	gob.Register(&Flash{})
}

func sessAddFlashN(s *sessions.Session, messages []string) {
	sessAddFlash(s, messages, FlashNormal)
}

func sessAddFlashW(s *sessions.Session, messages []string) {
	sessAddFlash(s, messages, FlashWarning)
}

func sessAddFlash(s *sessions.Session, messages []string, flashT FlashType) {
	if len(messages) == 0 {
		return
	}
	for i, message := range messages {
		if i > 6 {
			break
		}
		s.AddFlash(Flash{
			Message: message,
			Type:    flashT,
		})
	}
}

func sessLogSave(s *sessions.Session, w http.ResponseWriter, r *http.Request) {
	if err := s.Save(r, w); err != nil {
		log.Printf("error saving session: %v\n", err)
	}
}

// ## begin validation
// ## begin validation
// ## begin validation

var (
	errValiNoUsername        = errors.New("please enter a username")
	errValiPasswordAllFields = errors.New("please enter the password twice")
	errValiPasswordsNotSame  = errors.New("passwords entered were not the same")
	errValiKeysAllFields     = errors.New("please enter the api key and secret")
)

func validateUsername(username string) error {
	if username == "" {
		return errValiNoUsername
	}
	return nil
}

func validatePasswords(pOne, pTwo string) error {
	if pOne == "" || pTwo == "" {
		return errValiPasswordAllFields
	}
	if !(pOne == pTwo) {
		return errValiPasswordsNotSame
	}
	return nil
}

func validateAPIKey(apiKey, secret string) error {
	if apiKey == "" || secret == "" {
		return errValiKeysAllFields
	}
	return nil
}
