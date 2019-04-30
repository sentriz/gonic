package main

import (
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/gorilla/securecookie"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/wader/gormstore"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler"
)

type middleware func(next http.HandlerFunc) http.HandlerFunc

func newChain(wares ...middleware) middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(wares) - 1; i >= 0; i-- {
				last = wares[i](last)
			}
			last(w, r)
		}
	}
}

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

func setSubsonicRoutes(cont handler.Controller, mux *http.ServeMux) {
	withWare := newChain(
		cont.WithLogging,
		cont.WithCORS,
		cont.WithValidSubsonicArgs,
	)
	// common
	mux.HandleFunc("/rest/download", withWare(cont.Stream))
	mux.HandleFunc("/rest/download.view", withWare(cont.Stream))
	mux.HandleFunc("/rest/stream", withWare(cont.Stream))
	mux.HandleFunc("/rest/stream.view", withWare(cont.Stream))
	mux.HandleFunc("/rest/getCoverArt", withWare(cont.GetCoverArt))
	mux.HandleFunc("/rest/getCoverArt.view", withWare(cont.GetCoverArt))
	mux.HandleFunc("/rest/getLicense", withWare(cont.GetLicence))
	mux.HandleFunc("/rest/getLicense.view", withWare(cont.GetLicence))
	mux.HandleFunc("/rest/ping", withWare(cont.Ping))
	mux.HandleFunc("/rest/ping.view", withWare(cont.Ping))
	mux.HandleFunc("/rest/scrobble", withWare(cont.Scrobble))
	mux.HandleFunc("/rest/scrobble.view", withWare(cont.Scrobble))
	mux.HandleFunc("/rest/getMusicFolders", withWare(cont.GetMusicFolders))
	mux.HandleFunc("/rest/getMusicFolders.view", withWare(cont.GetMusicFolders))
	// browse by tag
	mux.HandleFunc("/rest/getAlbum", withWare(cont.GetAlbum))
	mux.HandleFunc("/rest/getAlbum.view", withWare(cont.GetAlbum))
	mux.HandleFunc("/rest/getAlbumList2", withWare(cont.GetAlbumList))
	mux.HandleFunc("/rest/getAlbumList2.view", withWare(cont.GetAlbumList))
	mux.HandleFunc("/rest/getArtist", withWare(cont.GetArtist))
	mux.HandleFunc("/rest/getArtist.view", withWare(cont.GetArtist))
	mux.HandleFunc("/rest/getArtists", withWare(cont.GetArtists))
	mux.HandleFunc("/rest/getArtists.view", withWare(cont.GetArtists))
	// browse by folder
	mux.HandleFunc("/rest/getIndexes", withWare(cont.GetIndexes))
	mux.HandleFunc("/rest/getIndexes.view", withWare(cont.GetIndexes))
	mux.HandleFunc("/rest/getMusicDirectory", withWare(cont.GetMusicDirectory))
	mux.HandleFunc("/rest/getMusicDirectory.view", withWare(cont.GetMusicDirectory))
}

func setAdminRoutes(cont handler.Controller, mux *http.ServeMux) {
	sessionKey := []byte(cont.GetSetting("session_key"))
	if len(sessionKey) == 0 {
		sessionKey = securecookie.GenerateRandomKey(32)
		cont.SetSetting("session_key", string(sessionKey))
	}
	// create gormstore (and cleanup) for backend sessions
	cont.SStore = gormstore.New(cont.DB, []byte(sessionKey))
	go cont.SStore.PeriodicCleanup(1*time.Hour, nil)
	// using packr to bundle templates and static files
	box := packr.New("templates", "../../templates")
	layoutT := extendFromBox(nil, box, "layout.tmpl")
	userT := extendFromBox(layoutT, box, "user.tmpl")
	cont.Templates = map[string]*template.Template{
		"login":                 extendFromBox(layoutT, box, "pages/login.tmpl"),
		"home":                  extendFromBox(userT, box, "pages/home.tmpl"),
		"change_own_password":   extendFromBox(userT, box, "pages/change_own_password.tmpl"),
		"change_password":       extendFromBox(userT, box, "pages/change_password.tmpl"),
		"delete_user":           extendFromBox(userT, box, "pages/delete_user.tmpl"),
		"create_user":           extendFromBox(userT, box, "pages/create_user.tmpl"),
		"update_lastfm_api_key": extendFromBox(userT, box, "pages/update_lastfm_api_key.tmpl"),
	}
	withPublicWare := newChain(
		cont.WithLogging,
		cont.WithSession,
	)
	withUserWare := newChain(
		withPublicWare,
		cont.WithUserSession,
	)
	withAdminWare := newChain(
		withUserWare,
		cont.WithAdminSession,
	)
	server := http.FileServer(packr.New("static", "../../static"))
	mux.Handle("/admin/static/", http.StripPrefix("/admin/static/", server))
	mux.HandleFunc("/admin/login", withPublicWare(cont.ServeLogin))
	mux.HandleFunc("/admin/login_do", withPublicWare(cont.ServeLoginDo))
	mux.HandleFunc("/admin/logout", withUserWare(cont.ServeLogout))
	mux.HandleFunc("/admin/home", withUserWare(cont.ServeHome))
	mux.HandleFunc("/admin/change_own_password", withUserWare(cont.ServeChangeOwnPassword))
	mux.HandleFunc("/admin/change_own_password_do", withUserWare(cont.ServeChangeOwnPasswordDo))
	mux.HandleFunc("/admin/link_lastfm_do", withUserWare(cont.ServeLinkLastFMDo))
	mux.HandleFunc("/admin/unlink_lastfm_do", withUserWare(cont.ServeUnlinkLastFMDo))
	mux.HandleFunc("/admin/change_password", withAdminWare(cont.ServeChangePassword))
	mux.HandleFunc("/admin/change_password_do", withAdminWare(cont.ServeChangePasswordDo))
	mux.HandleFunc("/admin/delete_user", withAdminWare(cont.ServeDeleteUser))
	mux.HandleFunc("/admin/delete_user_do", withAdminWare(cont.ServeDeleteUserDo))
	mux.HandleFunc("/admin/create_user", withAdminWare(cont.ServeCreateUser))
	mux.HandleFunc("/admin/create_user_do", withAdminWare(cont.ServeCreateUserDo))
	mux.HandleFunc("/admin/update_lastfm_api_key", withAdminWare(cont.ServeUpdateLastFMAPIKey))
	mux.HandleFunc("/admin/update_lastfm_api_key_do", withAdminWare(cont.ServeUpdateLastFMAPIKeyDo))
}

func main() {
	address := ":6969"
	mux := http.NewServeMux()
	// create a new controller and pass a copy to both routes.
	// they will add more fields to their copy if they need them
	baseController := handler.Controller{DB: db.New()}
	setSubsonicRoutes(baseController, mux)
	setAdminRoutes(baseController, mux)
	server := &http.Server{
		Addr:         address,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	log.Printf("starting server at `%s`\n", address)
	err := server.ListenAndServe()
	if err != nil {
		log.Printf("when starting server: %v\n", err)
	}
}
