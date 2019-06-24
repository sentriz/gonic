package server

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/securecookie"
	"github.com/wader/gormstore"
)

var (
	tmplFuncMap = template.FuncMap{
		"humandate": humanize.Time,
	}
)

func extendFrom(tmpl *template.Template, key string) *template.Template {
	strT, ok := assets.String(key)
	if !ok {
		log.Fatalf("error when reading %q from assets", key)
	}
	if tmpl == nil {
		tmpl = template.New("layout").Funcs(tmplFuncMap)
	} else {
		tmpl = template.Must(tmpl.Clone())
	}
	newT, err := tmpl.Parse(strT)
	if err != nil {
		log.Fatalf("error when parsing template: %v", err)
	}
	return newT
}

func (s *Server) SetupAdmin() {
	sessionKey := []byte(s.GetSetting("session_key"))
	if len(sessionKey) == 0 {
		sessionKey = securecookie.GenerateRandomKey(32)
		s.SetSetting("session_key", string(sessionKey))
	}
	s.SessDB = gormstore.New(s.DB, sessionKey)
	go s.SessDB.PeriodicCleanup(time.Hour, nil)
	//
	layoutT := extendFrom(nil, "/templates/layout.tmpl")
	userT := extendFrom(layoutT, "/templates/user.tmpl")
	s.Templates = map[string]*template.Template{
		"login":                 extendFrom(layoutT, "/templates/pages/login.tmpl"),
		"home":                  extendFrom(userT, "/templates/pages/home.tmpl"),
		"change_own_password":   extendFrom(userT, "/templates/pages/change_own_password.tmpl"),
		"change_password":       extendFrom(userT, "/templates/pages/change_password.tmpl"),
		"delete_user":           extendFrom(userT, "/templates/pages/delete_user.tmpl"),
		"create_user":           extendFrom(userT, "/templates/pages/create_user.tmpl"),
		"update_lastfm_api_key": extendFrom(userT, "/templates/pages/update_lastfm_api_key.tmpl"),
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
}
