package server

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Masterminds/sprig" //nolint:typecheck
	"github.com/dustin/go-humanize"
	"github.com/gorilla/securecookie"
	"github.com/pkg/errors"
	"github.com/wader/gormstore"
)

var (
	partialsPaths = []string{
		"partials/box.tmpl",
	}
	layoutPaths = []string{
		"layouts/base.tmpl",
		"layouts/user.tmpl",
	}
	pagePaths = []string{
		"pages/change_own_password.tmpl",
		"pages/change_password.tmpl",
		"pages/create_user.tmpl",
		"pages/delete_user.tmpl",
		"pages/home.tmpl",
		"pages/login.tmpl",
		"pages/update_lastfm_api_key.tmpl",
	}
	imagePaths = []string{
		"images/favicon.ico",
		"images/gonic.png",
		"images/gone.png",
	}
	stylesheetPaths = []string{
		"stylesheets/main.css",
		"stylesheets/reset.css",
	}
)

type templateMap map[string]*template.Template

func parseFromPaths(assets *Assets, base *template.Template,
	paths []string, destination templateMap) error {
	for _, path := range paths {
		_, tmplBytes, err := assets.FindBytes(path)
		if err != nil {
			return errors.Wrapf(err, "getting template %q from assets", path)
		}
		tmplStr := string(tmplBytes)
		if destination != nil {
			// we have a destination. meaning this template is a page.
			// instead of parsing as usual, we need to clone and add to the
			// template map
			clone := template.Must(base.Clone())
			tmplKey := filepath.Base(path)
			destination[tmplKey] = template.Must(clone.Parse(tmplStr))
			continue
		}
		_ = template.Must(base.Parse(tmplStr))
	}
	return nil
}

func staticHandler(assets *Assets, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		modTime, reader, err := assets.Find(path)
		if err != nil {
			log.Printf("error getting file %q from assets: %v\n", path, err)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		_, name := filepath.Split(path)
		http.ServeContent(w, r, name, modTime, reader)
	}
}

func (s *Server) SetupAdmin() error {
	sessionKey := []byte(s.GetSetting("session_key"))
	if len(sessionKey) == 0 {
		sessionKey = securecookie.GenerateRandomKey(32)
		s.SetSetting("session_key", string(sessionKey))
	}
	s.SessDB = gormstore.New(s.DB, sessionKey)
	go s.SessDB.PeriodicCleanup(time.Hour, nil)
	//
	tmplBase := template.
		New("layout").
		Funcs(sprig.FuncMap()).
		Funcs(template.FuncMap{
			"humanDate": humanize.Time,
		})
	if err := parseFromPaths(
		s.assets, tmplBase, partialsPaths, nil); err != nil {
		return errors.Wrap(err, "parsing template partials")
	}
	if err := parseFromPaths(
		s.assets, tmplBase, layoutPaths, nil); err != nil {
		return errors.Wrap(err, "parsing template layouts")
	}
	s.Templates = make(templateMap)
	if err := parseFromPaths(
		s.assets, tmplBase, pagePaths, s.Templates); err != nil {
		return errors.Wrap(err, "parsing template pages for destination")
	}
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
	// begin static server
	for _, path := range append(imagePaths, stylesheetPaths...) {
		fullPath := filepath.Join("/admin/static", path)
		s.mux.HandleFunc(fullPath, staticHandler(s.assets, path))
	}
	// begin public routes (creates new session)
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
	return nil
}
