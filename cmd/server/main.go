package main

import (
	"log"
	"net/http"
	"time"

	"github.com/sentriz/gonic/db"
	"github.com/sentriz/gonic/handler"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/wader/gormstore"
)

var (
	dbCon = db.New()
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

func setSubsonicRoutes(mux *http.ServeMux) {
	cont := handler.Controller{
		DB: dbCon,
	}
	withWare := newChain(
		cont.LogConnection,
		cont.EnableCORS,
		cont.CheckParameters,
	)
	mux.HandleFunc("/rest/ping", withWare(cont.Ping))
	mux.HandleFunc("/rest/ping.view", withWare(cont.Ping))
	mux.HandleFunc("/rest/stream", withWare(cont.Stream))
	mux.HandleFunc("/rest/stream.view", withWare(cont.Stream))
	mux.HandleFunc("/rest/download", withWare(cont.Stream))
	mux.HandleFunc("/rest/download.view", withWare(cont.Stream))
	mux.HandleFunc("/rest/getCoverArt", withWare(cont.GetCoverArt))
	mux.HandleFunc("/rest/getCoverArt.view", withWare(cont.GetCoverArt))
	mux.HandleFunc("/rest/getArtists", withWare(cont.GetArtists))
	mux.HandleFunc("/rest/getArtists.view", withWare(cont.GetArtists))
	mux.HandleFunc("/rest/getArtist", withWare(cont.GetArtist))
	mux.HandleFunc("/rest/getArtist.view", withWare(cont.GetArtist))
	mux.HandleFunc("/rest/getAlbum", withWare(cont.GetAlbum))
	mux.HandleFunc("/rest/getAlbum.view", withWare(cont.GetAlbum))
	mux.HandleFunc("/rest/getMusicFolders", withWare(cont.GetMusicFolders))
	mux.HandleFunc("/rest/getMusicFolders.view", withWare(cont.GetMusicFolders))
	mux.HandleFunc("/rest/getAlbumList2", withWare(cont.GetAlbumList))
	mux.HandleFunc("/rest/getAlbumList2.view", withWare(cont.GetAlbumList))
	mux.HandleFunc("/rest/getLicense", withWare(cont.GetLicence))
	mux.HandleFunc("/rest/getLicense.view", withWare(cont.GetLicence))
}

func setAdminRoutes(mux *http.ServeMux) {
	cont := handler.Controller{
		DB:     dbCon,
		SStore: gormstore.New(dbCon, []byte("saynothinboys")),
	}
	withWare := newChain(
		cont.LogConnection,
		cont.EnableCORS,
	)
	server := http.FileServer(http.Dir("static"))
	mux.Handle("/admin/static/", http.StripPrefix("/admin/static/", server))
	mux.HandleFunc("/admin/login", withWare(cont.ServeLogin))
	mux.HandleFunc("/admin/authenticate", withWare(cont.ServeAuthenticate))
	mux.HandleFunc("/admin/home", withWare(cont.ServeHome))
	mux.HandleFunc("/admin/create_user", withWare(cont.ServeCreateUser))
	mux.HandleFunc("/admin/logout", withWare(cont.ServeLogout))
}

func main() {
	address := ":5000"
	mux := http.NewServeMux()
	setSubsonicRoutes(mux)
	setAdminRoutes(mux)
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
