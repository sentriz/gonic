package server

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/wader/gormstore"

	"go.senan.xyz/gonic/server/assets"
	"go.senan.xyz/gonic/server/ctrladmin"
	"go.senan.xyz/gonic/server/ctrlbase"
	"go.senan.xyz/gonic/server/ctrlsubsonic"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/jukebox"
	"go.senan.xyz/gonic/server/podcasts"
	"go.senan.xyz/gonic/server/scanner"
	"go.senan.xyz/gonic/server/scrobble"
	"go.senan.xyz/gonic/server/scrobble/lastfm"
	"go.senan.xyz/gonic/server/scrobble/listenbrainz"
)

type Options struct {
	DB             *db.DB
	MusicPath      string
	PodcastPath    string
	CachePath      string
	CoverCachePath string
	ProxyPrefix    string
	GenreSplit     string
}

type Server struct {
	scanner *scanner.Scanner
	jukebox *jukebox.Jukebox
	router  *mux.Router
	sessDB  *gormstore.Store
	podcast *podcasts.Podcasts
}

func New(opts Options) *Server {
	// ** begin sanitation
	opts.MusicPath = filepath.Clean(opts.MusicPath)
	opts.CachePath = filepath.Clean(opts.CachePath)
	opts.PodcastPath = filepath.Clean(opts.PodcastPath)
	// ** begin controllers
	scanner := scanner.New(opts.MusicPath, opts.DB, opts.GenreSplit)
	jukebox := jukebox.New(opts.MusicPath)
	// the base controller, it's fields/middlewares are embedded/used by the
	// other two admin ui and subsonic controllers
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
	//
	sessKey := opts.DB.GetOrCreateKey("session_key")
	sessDB := gormstore.New(opts.DB.DB, []byte(sessKey))
	sessDB.SessionOpts.HttpOnly = true
	sessDB.SessionOpts.SameSite = http.SameSiteLaxMode
	//
	pcInit := &podcasts.Podcasts{DB: opts.DB, PodcastBasePath: opts.PodcastPath}
	ctrlAdmin := ctrladmin.New(base, sessDB, pcInit)
	scrobblers := []scrobble.Scrobbler{
		&lastfm.Scrobbler{DB: opts.DB},
		&listenbrainz.Scrobbler{},
	}
	ctrlSubsonic := &ctrlsubsonic.Controller{
		Controller:     base,
		CachePath:      opts.CachePath,
		CoverCachePath: opts.CoverCachePath,
		Jukebox:        jukebox,
		Scrobblers:     scrobblers,
		Podcasts:       pcInit,
	}
	setupMisc(r, base)
	setupAdmin(r.PathPrefix("/admin").Subrouter(), ctrlAdmin)
	setupSubsonic(r.PathPrefix("/rest").Subrouter(), ctrlSubsonic)
	//
	return &Server{
		scanner: scanner,
		jukebox: jukebox,
		router:  r,
		sessDB:  sessDB,
		podcast: &podcasts.Podcasts{DB: opts.DB, PodcastBasePath: opts.PodcastPath},
	}
}

func setupMisc(r *mux.Router, ctrl *ctrlbase.Controller) {
	r.HandleFunc("/",
		func(w http.ResponseWriter, r *http.Request) {
			// make the admin page the default
			http.Redirect(w, r, ctrl.Path("/admin/home"), http.StatusSeeOther)
		})
	r.HandleFunc("/musicFolderSettings.view",
		func(w http.ResponseWriter, r *http.Request) {
			// jamstash seems to call "musicFolderSettings.view" to start a scan. notice
			// that there is no "/rest/" prefix, so i doesn't fit in with the nice router,
			// custom handler, middleware. etc setup that we've got in `SetupSubsonic()`.
			// instead lets redirect to down there and use the scan endpoint
			redirectTo := fmt.Sprintf("/rest/startScan.view?%s", r.URL.Query().Encode())
			http.Redirect(w, r, ctrl.Path(redirectTo), http.StatusSeeOther)
		})
}

func setupAdmin(r *mux.Router, ctrl *ctrladmin.Controller) {
	// ** begin public routes (creates session)
	r.Use(ctrl.WithSession)
	r.Handle("/login", ctrl.H(ctrl.ServeLogin))
	r.Handle("/login_do", ctrl.HR(ctrl.ServeLoginDo)) // "raw" handler, updates session
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
	routUser.Handle("/logout", ctrl.HR(ctrl.ServeLogout)) // "raw" handler, updates session
	routUser.Handle("/home", ctrl.H(ctrl.ServeHome))
	routUser.Handle("/change_own_username", ctrl.H(ctrl.ServeChangeOwnUsername))
	routUser.Handle("/change_own_username_do", ctrl.H(ctrl.ServeChangeOwnUsernameDo))
	routUser.Handle("/change_own_password", ctrl.H(ctrl.ServeChangeOwnPassword))
	routUser.Handle("/change_own_password_do", ctrl.H(ctrl.ServeChangeOwnPasswordDo))
	routUser.Handle("/link_lastfm_do", ctrl.H(ctrl.ServeLinkLastFMDo))
	routUser.Handle("/unlink_lastfm_do", ctrl.H(ctrl.ServeUnlinkLastFMDo))
	routUser.Handle("/link_listenbrainz_do", ctrl.H(ctrl.ServeLinkListenBrainzDo))
	routUser.Handle("/unlink_listenbrainz_do", ctrl.H(ctrl.ServeUnlinkListenBrainzDo))
	routUser.Handle("/upload_playlist_do", ctrl.H(ctrl.ServeUploadPlaylistDo))
	routUser.Handle("/delete_playlist_do", ctrl.H(ctrl.ServeDeletePlaylistDo))
	routUser.Handle("/create_transcode_pref_do", ctrl.H(ctrl.ServeCreateTranscodePrefDo))
	routUser.Handle("/delete_transcode_pref_do", ctrl.H(ctrl.ServeDeleteTranscodePrefDo))
	if ctrl.Podcasts.PodcastBasePath != "" {
		routUser.Handle("/add_podcast_do", ctrl.H(ctrl.ServePodcastAddDo))
		routUser.Handle("/delete_podcast_do", ctrl.H(ctrl.ServePodcastDeleteDo))
	}
	// ** begin admin routes (if session is valid, and is admin)
	routAdmin := routUser.NewRoute().Subrouter()
	routAdmin.Use(ctrl.WithAdminSession)
	routAdmin.Handle("/change_username", ctrl.H(ctrl.ServeChangeUsername))
	routAdmin.Handle("/change_username_do", ctrl.H(ctrl.ServeChangeUsernameDo))
	routAdmin.Handle("/change_password", ctrl.H(ctrl.ServeChangePassword))
	routAdmin.Handle("/change_password_do", ctrl.H(ctrl.ServeChangePasswordDo))
	routAdmin.Handle("/delete_user", ctrl.H(ctrl.ServeDeleteUser))
	routAdmin.Handle("/delete_user_do", ctrl.H(ctrl.ServeDeleteUserDo))
	routAdmin.Handle("/create_user", ctrl.H(ctrl.ServeCreateUser))
	routAdmin.Handle("/create_user_do", ctrl.H(ctrl.ServeCreateUserDo))
	routAdmin.Handle("/update_lastfm_api_key", ctrl.H(ctrl.ServeUpdateLastFMAPIKey))
	routAdmin.Handle("/update_lastfm_api_key_do", ctrl.H(ctrl.ServeUpdateLastFMAPIKeyDo))
	routAdmin.Handle("/start_scan_inc_do", ctrl.H(ctrl.ServeStartScanIncDo))
	routAdmin.Handle("/start_scan_full_do", ctrl.H(ctrl.ServeStartScanFullDo))
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
	r.Handle("/createPlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeCreatePlaylist))
	r.Handle("/updatePlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeUpdatePlaylist))
	r.Handle("/deletePlaylist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeletePlaylist))
	r.Handle("/savePlayQueue{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSavePlayQueue))
	r.Handle("/getPlayQueue{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetPlayQueue))
	r.Handle("/getSong{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetSong))
	r.Handle("/getRandomSongs{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetRandomSongs))
	r.Handle("/getSongsByGenre{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetSongsByGenre))
	r.Handle("/jukeboxControl{_:(?:\\.view)?}", ctrl.H(ctrl.ServeJukebox))
	// ** begin raw
	r.Handle("/download{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeDownload))
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
	r.Handle("/getGenres{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetGenres))
	r.Handle("/getArtistInfo{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtistInfo))
	// ** begin podcasts
	if ctrl.Podcasts.PodcastBasePath != "" {
		r.Handle("/getPodcasts{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetPodcasts))
		r.Handle("/downloadPodcastEpisode{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDownloadPodcastEpisode))
		r.Handle("/createPodcastChannel{_:(?:\\.view)?}", ctrl.H(ctrl.ServeCreatePodcastChannel))
		r.Handle("/refreshPodcasts{_:(?:\\.view)?}", ctrl.H(ctrl.ServeRefreshPodcasts))
		r.Handle("/deletePodcastChannel{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeletePodcastChannel))
		r.Handle("/deletePodcastEpisode{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeletePodcastEpisode))
	}
	// middlewares should be run for not found handler
	// https://github.com/gorilla/mux/issues/416
	notFoundHandler := ctrl.H(ctrl.ServeNotFound)
	notFoundRoute := r.NewRoute().Handler(notFoundHandler)
	r.NotFoundHandler = notFoundRoute.GetHandler()
}

type (
	FuncExecute   func() error
	FuncInterrupt func(error)
)

func (s *Server) StartHTTP(listenAddr string) (FuncExecute, FuncInterrupt) {
	list := &http.Server{
		Addr:         listenAddr,
		Handler:      s.router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 80 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return func() error {
			log.Print("starting job 'http'\n")
			return list.ListenAndServe()
		}, func(_ error) {
			// stop job
			_ = list.Close()
		}
}

func (s *Server) StartScanTicker(dur time.Duration) (FuncExecute, FuncInterrupt) {
	ticker := time.NewTicker(dur)
	done := make(chan struct{})
	waitFor := func() error {
		for {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				if err := s.scanner.Start(scanner.ScanOptions{}); err != nil {
					log.Printf("error scanning: %v", err)
				}
			}
		}
	}
	return func() error {
			log.Printf("starting job 'scan timer'\n")
			return waitFor()
		}, func(_ error) {
			// stop job
			ticker.Stop()
			done <- struct{}{}
		}
}

func (s *Server) StartJukebox() (FuncExecute, FuncInterrupt) {
	return func() error {
			log.Printf("starting job 'jukebox'\n")
			return s.jukebox.Listen()
		}, func(_ error) {
			// stop job
			s.jukebox.Quit()
		}
}

func (s *Server) StartPodcastRefresher(dur time.Duration) (FuncExecute, FuncInterrupt) {
	ticker := time.NewTicker(dur)
	done := make(chan struct{})
	waitFor := func() error {
		for {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				if err := s.podcast.RefreshPodcasts(0, true); err != nil {
					log.Printf("failed to refresh some feeds: %s", err)
				}
			}
		}
	}
	return func() error {
			log.Printf("starting job 'podcast refresher'\n")
			return waitFor()
		}, func(_ error) {
			// stop job
			ticker.Stop()
			done <- struct{}{}
		}
}

func (s *Server) StartSessionClean(dur time.Duration) (FuncExecute, FuncInterrupt) {
	ticker := time.NewTicker(dur)
	done := make(chan struct{})
	waitFor := func() error {
		for {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				s.sessDB.Cleanup()
			}
		}
	}
	return func() error {
			log.Printf("starting job 'session clean'\n")
			return waitFor()
		}, func(_ error) {
			// stop job
			ticker.Stop()
			done <- struct{}{}
		}
}
