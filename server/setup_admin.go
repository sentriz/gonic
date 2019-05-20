package server

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/securecookie"
	"github.com/wader/gormstore"
)

func extendFromBox(tmpl *template.Template, box *packr.Box, key string) *template.Template {
	strT, err := box.FindString(key)
	if err != nil {
		log.Fatalf("error when reading template from box: %v", err)
	}
	if tmpl == nil {
		tmpl = template.New("layout")
	} else {
		tmpl = template.Must(tmpl.Clone())
	}
	newT, err := tmpl.Parse(strT)
	if err != nil {
		log.Fatalf("error when parsing template template: %v", err)
	}
	return newT
}

func (s *Server) setupAdmin() {
	sessionKey := []byte(s.GetSetting("session_key"))
	if len(sessionKey) == 0 {
		sessionKey = securecookie.GenerateRandomKey(32)
		s.SetSetting("session_key", string(sessionKey))
	}
	// create gormstore (and cleanup) for backend sessions
	s.SessDB = gormstore.New(s.DB, []byte(sessionKey))
	go s.SessDB.PeriodicCleanup(1*time.Hour, nil)
	// using packr to bundle templates and static files
	box := packr.New("templates", "./templates")
	layoutT := extendFromBox(nil, box, "layout.tmpl")
	userT := extendFromBox(layoutT, box, "user.tmpl")
	s.Templates = map[string]*template.Template{
		"login":                 extendFromBox(layoutT, box, "pages/login.tmpl"),
		"home":                  extendFromBox(userT, box, "pages/home.tmpl"),
		"change_own_password":   extendFromBox(userT, box, "pages/change_own_password.tmpl"),
		"change_password":       extendFromBox(userT, box, "pages/change_password.tmpl"),
		"delete_user":           extendFromBox(userT, box, "pages/delete_user.tmpl"),
		"create_user":           extendFromBox(userT, box, "pages/create_user.tmpl"),
		"update_lastfm_api_key": extendFromBox(userT, box, "pages/update_lastfm_api_key.tmpl"),
	}
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
	server := http.FileServer(packr.New("static", "./static"))
	s.mux.Handle("/admin/static/", http.StripPrefix("/admin/static/", server))
	s.mux.HandleFunc("/admin/login", withPublicWare(s.ServeLogin))
	s.mux.HandleFunc("/admin/login_do", withPublicWare(s.ServeLoginDo))
	s.mux.HandleFunc("/admin/logout", withUserWare(s.ServeLogout))
	s.mux.HandleFunc("/admin/home", withUserWare(s.ServeHome))
	s.mux.HandleFunc("/admin/change_own_password", withUserWare(s.ServeChangeOwnPassword))
	s.mux.HandleFunc("/admin/change_own_password_do", withUserWare(s.ServeChangeOwnPasswordDo))
	s.mux.HandleFunc("/admin/link_lastfm_do", withUserWare(s.ServeLinkLastFMDo))
	s.mux.HandleFunc("/admin/unlink_lastfm_do", withUserWare(s.ServeUnlinkLastFMDo))
	s.mux.HandleFunc("/admin/change_password", withAdminWare(s.ServeChangePassword))
	s.mux.HandleFunc("/admin/change_password_do", withAdminWare(s.ServeChangePasswordDo))
	s.mux.HandleFunc("/admin/delete_user", withAdminWare(s.ServeDeleteUser))
	s.mux.HandleFunc("/admin/delete_user_do", withAdminWare(s.ServeDeleteUserDo))
	s.mux.HandleFunc("/admin/create_user", withAdminWare(s.ServeCreateUser))
	s.mux.HandleFunc("/admin/create_user_do", withAdminWare(s.ServeCreateUserDo))
	s.mux.HandleFunc("/admin/update_lastfm_api_key", withAdminWare(s.ServeUpdateLastFMAPIKey))
	s.mux.HandleFunc("/admin/update_lastfm_api_key_do", withAdminWare(s.ServeUpdateLastFMAPIKeyDo))
}
