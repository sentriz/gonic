package server

import (
	"bytes"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/sprig" //nolint:typecheck
	"github.com/dustin/go-humanize"
	"github.com/gorilla/securecookie"
	"github.com/wader/gormstore"
)

const (
	prefixLayouts  = "layouts"
	prefixPages    = "pages"
	prefixPartials = "partials"
	prefixStatic   = "static"
)

type tmplMap map[string]*template.Template

// prefixDo runs a given callback for every path in our assets with
// the given prefix
func prefixDo(pre string, cb func(path string, asset *EmbeddedAsset)) {
	for path, asset := range assetBytes {
		if strings.HasPrefix(path, pre) {
			cb(path, asset)
		}
	}
}

// extendFromPaths /extends/ the given template for every asset
// with given prefix
func extendFromPaths(b *template.Template, p string) *template.Template {
	prefixDo(p, func(_ string, asset *EmbeddedAsset) {
		tmplStr := string(asset.Bytes)
		b = template.Must(b.Parse(tmplStr))
	})
	return b
}

// extendFromPaths /clones/ the given template for every asset
// with given prefix, extends it, and insert it into a new tmplMap
func pagesFromPaths(b *template.Template, p string) tmplMap {
	ret := tmplMap{}
	prefixDo(p, func(path string, asset *EmbeddedAsset) {
		tmplKey := filepath.Base(path)
		clone := template.Must(b.Clone())
		tmplStr := string(asset.Bytes)
		ret[tmplKey] = template.Must(clone.Parse(tmplStr))
	})
	return ret
}

func (s *Server) SetupAdmin() error {
	sessionKey := []byte(s.DB.GetSetting("session_key"))
	if len(sessionKey) == 0 {
		sessionKey = securecookie.GenerateRandomKey(32)
		s.DB.SetSetting("session_key", string(sessionKey))
	}
	s.SessDB = gormstore.New(s.DB.DB, sessionKey)
	go s.SessDB.PeriodicCleanup(time.Hour, nil)
	//
	tmplBase := template.
		New("layout").
		Funcs(sprig.FuncMap()).
		Funcs(template.FuncMap{
			"humanDate": humanize.Time,
		})
	tmplBase = extendFromPaths(tmplBase, prefixPartials)
	tmplBase = extendFromPaths(tmplBase, prefixLayouts)
	s.Templates = pagesFromPaths(tmplBase, prefixPages)
	// setup static server
	prefixDo(prefixStatic, func(path string, asset *EmbeddedAsset) {
		_, name := filepath.Split(path)
		route := filepath.Join("/admin/static", name)
		reader := bytes.NewReader(asset.Bytes)
		s.mux.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, name, asset.ModTime, reader)
		})
	})
	//
	withPublicWare := newChain(
		s.WithLogging,
		s.WithSession,
	)
	withUserWare := newChain(
		withPublicWare,
		s.WithUserSession,
	)
	withAdminWare := newChain(
		withUserWare,
		s.WithAdminSession,
	)
	// begin public routes (creates new session)
	// redirect / to /admin/home
	s.mux.HandleFunc("/", withPublicWare(s.ServeRedirectHome))
	s.mux.HandleFunc("/admin/login", withPublicWare(s.ServeLogin))
	s.mux.HandleFunc("/admin/login_do", withPublicWare(s.ServeLoginDo))
	// begin user routes (if session is valid)
	s.mux.HandleFunc("/admin/logout", withUserWare(s.ServeLogout))
	s.mux.HandleFunc("/admin/home", withUserWare(s.ServeHome))
	s.mux.HandleFunc("/admin/change_own_password", withUserWare(s.ServeChangeOwnPassword))
	s.mux.HandleFunc("/admin/change_own_password_do", withUserWare(s.ServeChangeOwnPasswordDo))
	s.mux.HandleFunc("/admin/link_lastfm_do", withUserWare(s.ServeLinkLastFMDo))
	s.mux.HandleFunc("/admin/unlink_lastfm_do", withUserWare(s.ServeUnlinkLastFMDo))
	// begin admin routes (if session is valid, and is admin)
	s.mux.HandleFunc("/admin/change_password", withAdminWare(s.ServeChangePassword))
	s.mux.HandleFunc("/admin/change_password_do", withAdminWare(s.ServeChangePasswordDo))
	s.mux.HandleFunc("/admin/delete_user", withAdminWare(s.ServeDeleteUser))
	s.mux.HandleFunc("/admin/delete_user_do", withAdminWare(s.ServeDeleteUserDo))
	s.mux.HandleFunc("/admin/create_user", withAdminWare(s.ServeCreateUser))
	s.mux.HandleFunc("/admin/create_user_do", withAdminWare(s.ServeCreateUserDo))
	s.mux.HandleFunc("/admin/update_lastfm_api_key", withAdminWare(s.ServeUpdateLastFMAPIKey))
	s.mux.HandleFunc("/admin/update_lastfm_api_key_do", withAdminWare(s.ServeUpdateLastFMAPIKeyDo))
	s.mux.HandleFunc("/admin/start_scan_do", withAdminWare(s.ServeStartScanDo))
	return nil
}
