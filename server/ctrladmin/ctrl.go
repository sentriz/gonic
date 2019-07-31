package ctrladmin

import (
	"encoding/gob"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/oxtoacart/bpool"
	"github.com/wader/gormstore"

	"senan.xyz/g/gonic/assets"
	"senan.xyz/g/gonic/model"
	"senan.xyz/g/gonic/server/ctrlbase"
	"senan.xyz/g/gonic/server/key"
)

func init() {
	gob.Register(&Flash{})
}

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

type Controller struct {
	*ctrlbase.Controller
	buffPool  *bpool.BufferPool
	templates map[string]*template.Template
	sessDB    *gormstore.Store
}

func New(base *ctrlbase.Controller) *Controller {
	sessionKey := []byte(base.DB.GetSetting("session_key"))
	if len(sessionKey) == 0 {
		sessionKey = securecookie.GenerateRandomKey(32)
		base.DB.SetSetting("session_key", string(sessionKey))
	}
	tmplBase := template.
		New("layout").
		Funcs(sprig.FuncMap()).
		Funcs(template.FuncMap{
			"humanDate": humanize.Time,
		})
	tmplBase = extendFromPaths(tmplBase, prefixPartials)
	tmplBase = extendFromPaths(tmplBase, prefixLayouts)
	return &Controller{
		Controller: base,
		buffPool:   bpool.NewBufferPool(64),
		templates:  pagesFromPaths(tmplBase, prefixPages),
		sessDB:     gormstore.New(base.DB.DB, sessionKey),
	}
}

type templateData struct {
	// common
	Flashes []interface{}
	User    *model.User
	// home
	AlbumCount    int
	ArtistCount   int
	TrackCount    int
	RequestRoot   string
	RecentFolders []*model.Album
	AllUsers      []*model.User
	LastScanTime  time.Time
	IsScanning    bool
	//
	CurrentLastFMAPIKey    string
	CurrentLastFMAPISecret string
	SelectedUser           *model.User
}

type adminHandler func(w http.ResponseWriter, r *http.Request) *Response

type Response struct {
	// code is 200
	template string
	data     *templateData
	// code is 303
	redirect string
	// code is >= 400
	code int
	err  string
}

func (c *Controller) H(h adminHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := h(w, r)
		if resp.redirect != "" {
			http.Redirect(w, r, resp.redirect, http.StatusSeeOther)
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
		session := r.Context().Value(key.Session).(*sessions.Session)
		resp.data.Flashes = session.Flashes()
		if err := session.Save(r, w); err != nil {
			http.Error(w, fmt.Sprintf("saving session: %v", err), 500)
			return
		}
		resp.data.User, _ = r.Context().Value(key.User).(*model.User)
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
		if _, err := buff.WriteTo(w); err != nil {
			log.Printf("error writing to response buffer: %v\n", err)
		}
	})
}

// ## begin utilities
// ## begin utilities
// ## begin utilities

func firstExisting(or string, strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return or
}

func sessLogSave(w http.ResponseWriter, r *http.Request, s *sessions.Session) {
	if err := s.Save(r, w); err != nil {
		log.Printf("error saving session: %v\n", err)
	}
}

type Flash struct {
	Message string
	Type    string
}

func sessAddFlashW(message string, s *sessions.Session) {
	s.AddFlash(Flash{
		Message: message,
		Type:    "warning",
	})
}

func sessAddFlashWf(message string, s *sessions.Session, a ...interface{}) {
	sessAddFlashW(fmt.Sprintf(message, a...), s)
}

func sessAddFlashN(message string, s *sessions.Session) {
	s.AddFlash(Flash{
		Message: message,
		Type:    "normal",
	})
}

func sessAddFlashNf(message string, s *sessions.Session, a ...interface{}) {
	sessAddFlashN(fmt.Sprintf(message, a...), s)
}

// ## begin validation
// ## begin validation
// ## begin validation

func validateUsername(username string) error {
	if username == "" {
		return fmt.Errorf("please enter the username")
	}
	return nil
}

func validatePasswords(pOne, pTwo string) error {
	if pOne == "" || pTwo == "" {
		return fmt.Errorf("please enter the password twice")
	}
	if !(pOne == pTwo) {
		return fmt.Errorf("the two passwords entered were not the same")
	}
	return nil
}

func validateAPIKey(apiKey, secret string) error {
	if apiKey == "" || secret == "" {
		return fmt.Errorf("please enter both the api key and secret")
	}
	return nil
}
