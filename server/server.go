package server

import (
	"bytes"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"

	"senan.xyz/g/gonic/assets"
	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/server/ctrladmin"
	"senan.xyz/g/gonic/server/ctrlbase"
	"senan.xyz/g/gonic/server/ctrlsubsonic"
)

type Server struct {
	*http.Server
	router   *mux.Router
	ctrlBase *ctrlbase.Controller
}

func New(db *db.DB, musicPath string, listenAddr string) *Server {
	ctrlBase := &ctrlbase.Controller{
		DB:        db,
		MusicPath: musicPath,
	}
	router := mux.NewRouter()
	// jamstash seems to call "musicFolderSettings.view" to start a scan. notice
	// that there is no "/rest/" prefix, so i doesn't fit in with the nice router,
	// custom handler, middleware. etc setup that we've got in `SetupSubsonic()`.
	// instead lets redirect to down there and use the scan endpoint
	router.HandleFunc("/musicFolderSettings.view", func(w http.ResponseWriter, r *http.Request) {
		oldParams := r.URL.Query().Encode()
		redirectTo := "/rest/startScan.view?" + oldParams
		http.Redirect(w, r, redirectTo, http.StatusMovedPermanently)
	})
	router.Use(ctrlBase.WithLogging)
	router.Use(ctrlBase.WithCORS)
	server := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	return &Server{
		Server:   server,
		router:   router,
		ctrlBase: ctrlBase,
	}
}

func (s *Server) SetupAdmin() error {
	ctrl := ctrladmin.New(s.ctrlBase)
	// TODO: remove all the H()s
	routPublic := s.router.PathPrefix("/admin").Subrouter()
	routPublic.Use(ctrl.WithSession)
	routPublic.Handle("/login", ctrl.H(ctrl.ServeLogin))
	routPublic.Handle("/login_do", ctrl.H(ctrl.ServeLoginDo))
	assets.PrefixDo("static", func(path string, asset *assets.EmbeddedAsset) {
		_, name := filepath.Split(path)
		route := filepath.Join("/static", name)
		reader := bytes.NewReader(asset.Bytes)
		routPublic.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, name, asset.ModTime, reader)
		})
	})
	//
	// begin user routes (if session is valid)
	routUser := routPublic.NewRoute().Subrouter()
	routUser.Use(ctrl.WithUserSession)
	routUser.Handle("/logout", ctrl.H(ctrl.ServeLogout))
	routUser.Handle("/home", ctrl.H(ctrl.ServeHome))
	routUser.Handle("/change_own_password", ctrl.H(ctrl.ServeChangeOwnPassword))
	routUser.Handle("/change_own_password_do", ctrl.H(ctrl.ServeChangeOwnPasswordDo))
	routUser.Handle("/link_lastfm_do", ctrl.H(ctrl.ServeLinkLastFMDo))
	routUser.Handle("/unlink_lastfm_do", ctrl.H(ctrl.ServeUnlinkLastFMDo))
	// begin admin routes (if session is valid, and is admin)
	routAdmin := routUser.NewRoute().Subrouter()
	routAdmin.Use(ctrl.WithAdminSession)
	routAdmin.Handle("/change_password", ctrl.H(ctrl.ServeChangePassword))
	routAdmin.Handle("/change_password_do", ctrl.H(ctrl.ServeChangePasswordDo))
	routAdmin.Handle("/delete_user", ctrl.H(ctrl.ServeDeleteUser))
	routAdmin.Handle("/delete_user_do", ctrl.H(ctrl.ServeDeleteUserDo))
	routAdmin.Handle("/create_user", ctrl.H(ctrl.ServeCreateUser))
	routAdmin.Handle("/create_user_do", ctrl.H(ctrl.ServeCreateUserDo))
	routAdmin.Handle("/update_lastfm_api_key", ctrl.H(ctrl.ServeUpdateLastFMAPIKey))
	routAdmin.Handle("/update_lastfm_api_key_do", ctrl.H(ctrl.ServeUpdateLastFMAPIKeyDo))
	routAdmin.Handle("/start_scan_do", ctrl.H(ctrl.ServeStartScanDo))
	return nil
}

func (s *Server) SetupSubsonic() error {
	ctrl := ctrlsubsonic.New(s.ctrlBase)
	rout := s.router.PathPrefix("/rest").Subrouter()
	rout.Use(ctrl.WithValidSubsonicArgs)
	rout.NotFoundHandler = ctrl.H(ctrl.ServeNotFound)
	// common
	rout.Handle("/getLicense{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetLicence))
	rout.Handle("/getMusicFolders{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetMusicFolders))
	rout.Handle("/getScanStatus{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetScanStatus))
	rout.Handle("/ping{_:(?:\\.view)?}", ctrl.H(ctrl.ServePing))
	rout.Handle("/scrobble{_:(?:\\.view)?}", ctrl.H(ctrl.ServeScrobble))
	rout.Handle("/startScan{_:(?:\\.view)?}", ctrl.H(ctrl.ServeStartScan))
	// raw
	rout.Handle("/download{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeStream))
	rout.Handle("/getCoverArt{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeGetCoverArt))
	rout.Handle("/stream{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeStream))
	// browse by tag
	rout.Handle("/getAlbum{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbum))
	rout.Handle("/getAlbumList2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbumListTwo))
	rout.Handle("/getArtist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtist))
	rout.Handle("/getArtists{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtists))
	rout.Handle("/search3{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSearchThree))
	// browse by folder
	rout.Handle("/getIndexes{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetIndexes))
	rout.Handle("/getMusicDirectory{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetMusicDirectory))
	rout.Handle("/getAlbumList{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbumList))
	rout.Handle("/search2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSearchTwo))
	return nil
}
