package server

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/securecookie"
	"github.com/pkg/errors"
	"github.com/shurcooL/httpfs/html/vfstemplate"
	"github.com/shurcooL/httpfs/path/vfspath"
	"github.com/wader/gormstore"
)

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
	const (
		layoutDir  = "/templates/layouts/*"
		partialDir = "/templates/partials/*"
		pageDir    = "/templates/pages/*"
	)
	tmplLayouts, err := vfstemplate.ParseGlob(assets, tmplBase, layoutDir)
	if err != nil {
		return errors.Wrap(err, "parsing layouts")
	}
	tmplPartials, err := vfstemplate.ParseGlob(assets, tmplLayouts, partialDir)
	if err != nil {
		return errors.Wrap(err, "parsing partials")
	}
	pages, err := vfspath.Glob(assets, pageDir)
	if err != nil {
		return errors.Wrap(err, "parsing pages")
	}
	s.Templates = make(map[string]*template.Template)
	for _, page := range pages {
		tmplStr, ok := assets.String(page)
		if !ok {
			return fmt.Errorf("getting template %q from assets: %v\n", page)
		}
		tmplBaseClone := template.Must(tmplPartials.Clone())
		tmplWithPage := template.Must(tmplBaseClone.Parse(tmplStr))
		tmplWithPartial := template.Must(tmplWithPage.Parse(tmplStr))
		shortName := filepath.Base(page)
		s.Templates[shortName] = tmplWithPartial
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
	s.mux.Handle("/admin/static/", http.StripPrefix("/admin",
		http.FileServer(assets),
	))
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
