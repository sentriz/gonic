package server

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"

	"senan.xyz/g/gonic/assets"
	"senan.xyz/g/gonic/db"
	"senan.xyz/g/gonic/scanner"
	"senan.xyz/g/gonic/server/ctrladmin"
	"senan.xyz/g/gonic/server/ctrlbase"
	"senan.xyz/g/gonic/server/ctrlsubsonic"
)

type Options struct {
	DB           *db.DB
	MusicPath    string
	ListenAddr   string
	ScanInterval time.Duration
	ProxyPrefix  string
}

type Server struct {
	*http.Server
	scanner      *scanner.Scanner
	scanInterval time.Duration
}

func New(opts Options) *Server {
	// ** begin sanitation
	opts.MusicPath = filepath.Clean(opts.MusicPath)
	// ** begin controllers
	scanner := scanner.New(opts.DB, opts.MusicPath)
	base := &ctrlbase.Controller{
		DB:          opts.DB,
		MusicPath:   opts.MusicPath,
		ProxyPrefix: opts.ProxyPrefix,
		Scanner:     scanner,
	}
	// router with common wares for admin / subsonic
	r := mux.NewRouter()
	r.Use(base.WithLogging)
	r.Use(base.WithCORS)
	setupMisc(r, base)
	setupAdmin(r.PathPrefix("/admin").Subrouter(), ctrladmin.New(base))
	setupSubsonic(r.PathPrefix("/rest").Subrouter(), ctrlsubsonic.New(base))
	//
	server := &http.Server{
		Addr:         opts.ListenAddr,
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	return &Server{
		Server:       server,
		scanner:      scanner,
		scanInterval: opts.ScanInterval,
	}
}

func setupMisc(r *mux.Router, ctrl *ctrlbase.Controller) {
	r.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			// make the admin page the default
			http.Redirect(w, r, ctrl.Path("/admin/home"), http.StatusMovedPermanently)
		})
	r.HandleFunc("/musicFolderSettings.view",
		func(w http.ResponseWriter, r *http.Request) {
			// jamstash seems to call "musicFolderSettings.view" to start a scan. notice
			// that there is no "/rest/" prefix, so i doesn't fit in with the nice router,
			// custom handler, middleware. etc setup that we've got in `SetupSubsonic()`.
			// instead lets redirect to down there and use the scan endpoint
			redirectTo := fmt.Sprintf("/rest/startScan.view?%s", r.URL.Query().Encode())
			http.Redirect(w, r, ctrl.Path(redirectTo), http.StatusMovedPermanently)
		})
}

func setupAdmin(r *mux.Router, ctrl *ctrladmin.Controller) {
	// ** begin public routes (creates session)
	r.Use(ctrl.WithSession)
	r.Handle("/login", ctrl.H(ctrl.ServeLogin))
	r.HandleFunc("/login_do", ctrl.ServeLoginDo) // "raw" handler, updates session
	assets.PrefixDo("static", func(path string, asset *assets.EmbeddedAsset) {
		_, name := filepath.Split(path)
		route := filepath.Join("/static", name)
		reader := bytes.NewReader(asset.Bytes)
		r.HandleFunc(route, func(w http.ResponseWriter, r *http.Request) {
			http.ServeContent(w, r, name, asset.ModTime, reader)
		})
	})
	// ** begin user routes (if session is valid)
	routUser := r.NewRoute().Subrouter()
	routUser.Use(ctrl.WithUserSession)
	routUser.HandleFunc("/logout", ctrl.ServeLogout) // "raw" handler, updates session
	routUser.Handle("/home", ctrl.H(ctrl.ServeHome))
	routUser.Handle("/change_own_password", ctrl.H(ctrl.ServeChangeOwnPassword))
	routUser.Handle("/change_own_password_do", ctrl.H(ctrl.ServeChangeOwnPasswordDo))
	routUser.Handle("/link_lastfm_do", ctrl.H(ctrl.ServeLinkLastFMDo))
	routUser.Handle("/unlink_lastfm_do", ctrl.H(ctrl.ServeUnlinkLastFMDo))
	routUser.Handle("/upload_playlist_do", ctrl.H(ctrl.ServeUploadPlaylistDo))
	// ** begin admin routes (if session is valid, and is admin)
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
	// middlewares should be run for not found handler
	// https://github.com/gorilla/mux/issues/416
	notFoundHandler := ctrl.H(ctrl.ServeNotFound)
	notFoundRoute := r.NewRoute().Handler(notFoundHandler)
	r.NotFoundHandler = notFoundRoute.GetHandler()
}

func setupSubsonic(r *mux.Router, ctrl *ctrlsubsonic.Controller) {
	r.Use(ctrl.WithParams)
	r.Use(ctrl.WithRequiredParams)
	r.Use(ctrl.WithUser)
	// ** begin common
	r.Handle("/getLicense{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetLicence))
	r.Handle("/getMusicFolders{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetMusicFolders))
	r.Handle("/getScanStatus{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetScanStatus))
	r.Handle("/ping{_:(?:\\.view)?}", ctrl.H(ctrl.ServePing))
	r.Handle("/scrobble{_:(?:\\.view)?}", ctrl.H(ctrl.ServeScrobble))
	r.Handle("/startScan{_:(?:\\.view)?}", ctrl.H(ctrl.ServeStartScan))
	r.Handle("/getUser{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetUser))
	r.Handle("/getPlaylists{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetPlaylists))
	r.Handle("/getPlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetPlaylist))
	r.Handle("/createPlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeUpdatePlaylist))
	r.Handle("/updatePlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeUpdatePlaylist))
	r.Handle("/deletePlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeletePlaylist))
	r.Handle("/savePlayQueue{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSavePlayQueue))
	r.Handle("/getPlayQueue{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetPlayQueue))
	r.Handle("/getSong{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetSong))
	r.Handle("/getRandomSongs{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetRandomSongs))
	// ** begin raw
	r.Handle("/download{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeStream))
	r.Handle("/getCoverArt{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeGetCoverArt))
	r.Handle("/stream{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeStream))
	// ** begin browse by tag
	r.Handle("/getAlbum{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbum))
	r.Handle("/getAlbumList2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbumListTwo))
	r.Handle("/getArtist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtist))
	r.Handle("/getArtists{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtists))
	r.Handle("/search3{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSearchThree))
	r.Handle("/getArtistInfo2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtistInfoTwo))
	// ** begin browse by folder
	r.Handle("/getIndexes{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetIndexes))
	r.Handle("/getMusicDirectory{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetMusicDirectory))
	r.Handle("/getAlbumList{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbumList))
	r.Handle("/search2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSearchTwo))
	// ** begin unimplemented
	r.Handle("/getGenres{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetGenres))
	// middlewares should be run for not found handler
	// https://github.com/gorilla/mux/issues/416
	notFoundHandler := ctrl.H(ctrl.ServeNotFound)
	notFoundRoute := r.NewRoute().Handler(notFoundHandler)
	r.NotFoundHandler = notFoundRoute.GetHandler()
}

func (s *Server) Start() error {
	if s.scanInterval > 0 {
		log.Printf("will be scanning at intervals of %s", s.scanInterval)
		ticker := time.NewTicker(s.scanInterval)
		go func() {
			for range ticker.C {
				if err := s.scanner.Start(); err != nil {
					log.Printf("error while scanner: %v", err)
				}
			}
		}()
	}
	return s.ListenAndServe()
}
