package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/sentriz/gormstore"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/jukebox"
	"go.senan.xyz/gonic/paths"
	"go.senan.xyz/gonic/podcasts"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/scanner/tags"
	"go.senan.xyz/gonic/scrobble"
	"go.senan.xyz/gonic/scrobble/lastfm"
	"go.senan.xyz/gonic/scrobble/listenbrainz"
	"go.senan.xyz/gonic/server/assets"
	"go.senan.xyz/gonic/server/ctrladmin"
	"go.senan.xyz/gonic/server/ctrlbase"
	"go.senan.xyz/gonic/server/ctrlsubsonic"
	"go.senan.xyz/gonic/transcode"
)

type Options struct {
	DB                *db.DB
	MusicPaths        paths.MusicPaths
	PodcastPath       string
	CachePath         string
	CoverCachePath    string
	ProxyPrefix       string
	GenreSplit        string
	PreferEmbeddedCue bool
	HTTPLog           bool
	JukeboxEnabled    bool
}

type Server struct {
	scanner *scanner.Scanner
	jukebox *jukebox.Jukebox
	router  *mux.Router
	sessDB  *gormstore.Store
	podcast *podcasts.Podcasts
}

func New(opts Options) (*Server, error) {
	tagger := &tags.TagReader{}

	scanner := scanner.New(opts.MusicPaths.Paths(), opts.DB, opts.GenreSplit, opts.PreferEmbeddedCue, tagger)
	base := &ctrlbase.Controller{
		DB:          opts.DB,
		ProxyPrefix: opts.ProxyPrefix,
		Scanner:     scanner,
	}

	// router with common wares for admin / subsonic
	r := mux.NewRouter()
	if opts.HTTPLog {
		r.Use(base.WithLogging)
	}
	r.Use(base.WithCORS)
	r.Use(handlers.RecoveryHandler(handlers.PrintRecoveryStack(true)))

	sessKey, err := opts.DB.GetSetting("session_key")
	if err != nil {
		return nil, fmt.Errorf("get session key: %w", err)
	}
	if sessKey == "" {
		if err := opts.DB.SetSetting("session_key", string(securecookie.GenerateRandomKey(32))); err != nil {
			return nil, fmt.Errorf("set session key: %w", err)
		}
	}

	sessDB := gormstore.New(opts.DB.DB, []byte(sessKey))
	sessDB.SessionOpts.HttpOnly = true
	sessDB.SessionOpts.SameSite = http.SameSiteLaxMode

	podcast := podcasts.New(opts.DB, opts.PodcastPath, tagger)

	cacheTranscoder := transcode.NewCachingTranscoder(
		transcode.NewFFmpegTranscoder(),
		opts.CachePath,
	)

	ctrlAdmin, err := ctrladmin.New(base, sessDB, podcast)
	if err != nil {
		return nil, fmt.Errorf("create admin controller: %w", err)
	}
	ctrlSubsonic := &ctrlsubsonic.Controller{
		Controller:     base,
		CachePath:      opts.CachePath,
		CoverCachePath: opts.CoverCachePath,
		PodcastsPath:   opts.PodcastPath,
		MusicPaths:     opts.MusicPaths,
		Scrobblers:     []scrobble.Scrobbler{&lastfm.Scrobbler{DB: opts.DB}, &listenbrainz.Scrobbler{}},
		Podcasts:       podcast,
		Transcoder:     cacheTranscoder,
	}

	setupMisc(r, base)
	setupAdmin(r.PathPrefix("/admin").Subrouter(), ctrlAdmin)
	setupSubsonic(r.PathPrefix("/rest").Subrouter(), ctrlSubsonic)

	server := &Server{
		scanner: scanner,
		router:  r,
		sessDB:  sessDB,
		podcast: podcast,
	}

	if opts.JukeboxEnabled {
		jukebox := jukebox.New()
		ctrlSubsonic.Jukebox = jukebox
		server.jukebox = jukebox
	}

	return server, nil
}

func setupMisc(r *mux.Router, ctrl *ctrlbase.Controller) {
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		adminHome := ctrl.Path("/admin/home")
		http.Redirect(w, r, adminHome, http.StatusSeeOther)
	})
	// misc subsonic routes without /rest prefix
	r.HandleFunc("/settings.view", func(w http.ResponseWriter, r *http.Request) {
		adminHome := ctrl.Path("/admin/home")
		http.Redirect(w, r, adminHome, http.StatusSeeOther)
	})
	r.HandleFunc("/musicFolderSettings.view", func(w http.ResponseWriter, r *http.Request) {
		restScan := ctrl.Path(fmt.Sprintf("/rest/startScan.view?%s", r.URL.Query().Encode()))
		http.Redirect(w, r, restScan, http.StatusSeeOther)
	})
	r.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, "OK")
	})
}

func setupAdmin(r *mux.Router, ctrl *ctrladmin.Controller) {

	// public routes (creates session)
	r.Use(ctrl.WithSession)
	r.Handle("/login", ctrl.H(ctrl.ServeLogin))
	r.Handle("/login_do", ctrl.HR(ctrl.ServeLoginDo)) // "raw" handler, updates session

	staticHandler := http.StripPrefix("/admin", http.FileServer(http.FS(assets.Static)))
	r.PathPrefix("/static").Handler(staticHandler)

	// user routes (if session is valid)
	routUser := r.NewRoute().Subrouter()
	routUser.Use(ctrl.WithUserSession)
	routUser.Handle("/logout", ctrl.HR(ctrl.ServeLogout)) // "raw" handler, updates session
	routUser.Handle("/home", ctrl.H(ctrl.ServeHome))
	routUser.Handle("/change_own_username", ctrl.H(ctrl.ServeChangeOwnUsername))
	routUser.Handle("/change_own_username_do", ctrl.H(ctrl.ServeChangeOwnUsernameDo))
	routUser.Handle("/change_own_password", ctrl.H(ctrl.ServeChangeOwnPassword))
	routUser.Handle("/change_own_password_do", ctrl.H(ctrl.ServeChangeOwnPasswordDo))
	routUser.Handle("/change_own_avatar", ctrl.H(ctrl.ServeChangeOwnAvatar))
	routUser.Handle("/change_own_avatar_do", ctrl.H(ctrl.ServeChangeOwnAvatarDo))
	routUser.Handle("/delete_own_avatar_do", ctrl.H(ctrl.ServeDeleteOwnAvatarDo))
	routUser.Handle("/link_lastfm_do", ctrl.H(ctrl.ServeLinkLastFMDo))
	routUser.Handle("/unlink_lastfm_do", ctrl.H(ctrl.ServeUnlinkLastFMDo))
	routUser.Handle("/link_listenbrainz_do", ctrl.H(ctrl.ServeLinkListenBrainzDo))
	routUser.Handle("/unlink_listenbrainz_do", ctrl.H(ctrl.ServeUnlinkListenBrainzDo))
	routUser.Handle("/upload_playlist_do", ctrl.H(ctrl.ServeUploadPlaylistDo))
	routUser.Handle("/delete_playlist_do", ctrl.H(ctrl.ServeDeletePlaylistDo))
	routUser.Handle("/create_transcode_pref_do", ctrl.H(ctrl.ServeCreateTranscodePrefDo))
	routUser.Handle("/delete_transcode_pref_do", ctrl.H(ctrl.ServeDeleteTranscodePrefDo))

	// admin routes (if session is valid, and is admin)
	routAdmin := routUser.NewRoute().Subrouter()
	routAdmin.Use(ctrl.WithAdminSession)
	routAdmin.Handle("/change_username", ctrl.H(ctrl.ServeChangeUsername))
	routAdmin.Handle("/change_username_do", ctrl.H(ctrl.ServeChangeUsernameDo))
	routAdmin.Handle("/change_password", ctrl.H(ctrl.ServeChangePassword))
	routAdmin.Handle("/change_password_do", ctrl.H(ctrl.ServeChangePasswordDo))
	routAdmin.Handle("/change_avatar", ctrl.H(ctrl.ServeChangeAvatar))
	routAdmin.Handle("/change_avatar_do", ctrl.H(ctrl.ServeChangeAvatarDo))
	routAdmin.Handle("/delete_avatar_do", ctrl.H(ctrl.ServeDeleteAvatarDo))
	routAdmin.Handle("/delete_user", ctrl.H(ctrl.ServeDeleteUser))
	routAdmin.Handle("/delete_user_do", ctrl.H(ctrl.ServeDeleteUserDo))
	routAdmin.Handle("/create_user", ctrl.H(ctrl.ServeCreateUser))
	routAdmin.Handle("/create_user_do", ctrl.H(ctrl.ServeCreateUserDo))
	routAdmin.Handle("/update_lastfm_api_key", ctrl.H(ctrl.ServeUpdateLastFMAPIKey))
	routAdmin.Handle("/update_lastfm_api_key_do", ctrl.H(ctrl.ServeUpdateLastFMAPIKeyDo))
	routAdmin.Handle("/start_scan_inc_do", ctrl.H(ctrl.ServeStartScanIncDo))
	routAdmin.Handle("/start_scan_full_do", ctrl.H(ctrl.ServeStartScanFullDo))
	routAdmin.Handle("/add_podcast_do", ctrl.H(ctrl.ServePodcastAddDo))
	routAdmin.Handle("/delete_podcast_do", ctrl.H(ctrl.ServePodcastDeleteDo))
	routAdmin.Handle("/download_podcast_do", ctrl.H(ctrl.ServePodcastDownloadDo))
	routAdmin.Handle("/update_podcast_do", ctrl.H(ctrl.ServePodcastUpdateDo))
	routAdmin.Handle("/add_internet_radio_station_do", ctrl.H(ctrl.ServeInternetRadioStationAddDo))
	routAdmin.Handle("/delete_internet_radio_station_do", ctrl.H(ctrl.ServeInternetRadioStationDeleteDo))
	routAdmin.Handle("/update_internet_radio_station_do", ctrl.H(ctrl.ServeInternetRadioStationUpdateDo))

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

	// common
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
	r.Handle("/getBookmarks{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetBookmarks))
	r.Handle("/createBookmark{_:(?:\\.view)?}", ctrl.H(ctrl.ServeCreateBookmark))
	r.Handle("/deleteBookmark{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeleteBookmark))
	r.Handle("/getTopSongs{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetTopSongs))
	r.Handle("/getSimilarSongs{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetSimilarSongs))
	r.Handle("/getSimilarSongs2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetSimilarSongsTwo))
	r.Handle("/getLyrics{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetLyrics))

	// raw
	r.Handle("/getCoverArt{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeGetCoverArt))
	r.Handle("/stream{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeStream))
	r.Handle("/download{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeStream))
	r.Handle("/getAvatar{_:(?:\\.view)?}", ctrl.HR(ctrl.ServeGetAvatar))

	// browse by tag
	r.Handle("/getAlbum{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbum))
	r.Handle("/getAlbumList2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbumListTwo))
	r.Handle("/getArtist{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtist))
	r.Handle("/getArtists{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtists))
	r.Handle("/search3{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSearchThree))
	r.Handle("/getArtistInfo2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtistInfoTwo))
	r.Handle("/getStarred2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetStarredTwo))

	// browse by folder
	r.Handle("/getIndexes{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetIndexes))
	r.Handle("/getMusicDirectory{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetMusicDirectory))
	r.Handle("/getAlbumList{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetAlbumList))
	r.Handle("/search2{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSearchTwo))
	r.Handle("/getGenres{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetGenres))
	r.Handle("/getArtistInfo{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetArtistInfo))
	r.Handle("/getStarred{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetStarred))

	// star / rating
	r.Handle("/star{_:(?:\\.view)?}", ctrl.H(ctrl.ServeStar))
	r.Handle("/unstar{_:(?:\\.view)?}", ctrl.H(ctrl.ServeUnstar))
	r.Handle("/setRating{_:(?:\\.view)?}", ctrl.H(ctrl.ServeSetRating))

	// podcasts
	r.Handle("/getPodcasts{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetPodcasts))
	r.Handle("/getNewestPodcasts{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetNewestPodcasts))
	r.Handle("/downloadPodcastEpisode{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDownloadPodcastEpisode))
	r.Handle("/createPodcastChannel{_:(?:\\.view)?}", ctrl.H(ctrl.ServeCreatePodcastChannel))
	r.Handle("/refreshPodcasts{_:(?:\\.view)?}", ctrl.H(ctrl.ServeRefreshPodcasts))
	r.Handle("/deletePodcastChannel{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeletePodcastChannel))
	r.Handle("/deletePodcastEpisode{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeletePodcastEpisode))

	// internet radio
	r.Handle("/getInternetRadioStations{_:(?:\\.view)?}", ctrl.H(ctrl.ServeGetInternetRadioStations))
	r.Handle("/createInternetRadioStation{_:(?:\\.view)?}", ctrl.H(ctrl.ServeCreateInternetRadioStation))
	r.Handle("/updateInternetRadioStation{_:(?:\\.view)?}", ctrl.H(ctrl.ServeUpdateInternetRadioStation))
	r.Handle("/deleteInternetRadioStation{_:(?:\\.view)?}", ctrl.H(ctrl.ServeDeleteInternetRadioStation))

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

func (s *Server) StartHTTP(listenAddr string, tlsCert string, tlsKey string) (FuncExecute, FuncInterrupt) {
	list := &http.Server{
		Addr:              listenAddr,
		Handler:           s.router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      80 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return func() error {
			log.Print("starting job 'http'\n")
			if tlsCert != "" && tlsKey != "" {
				return list.ListenAndServeTLS(tlsCert, tlsKey)
			}
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
				go func() {
					if _, err := s.scanner.ScanAndClean(scanner.ScanOptions{}); err != nil {
						log.Printf("error scanning: %v", err)
					}
				}()
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

func (s *Server) ScanAtStart() {
	if _, err := s.scanner.ScanAndClean(scanner.ScanOptions{}); err != nil {
		log.Printf("error scanning: %v", err)
	}
}

func (s *Server) StartScanWatcher() (FuncExecute, FuncInterrupt) {
	return func() error {
			log.Printf("starting job 'scan watcher'\n")
			return s.scanner.ExecuteWatch()
		}, func(_ error) {
			// stop job
			s.scanner.CancelWatch()
		}
}

func (s *Server) StartJukebox(mpvExtraArgs []string) (FuncExecute, FuncInterrupt) {
	var tempDir string
	return func() error {
			log.Printf("starting job 'jukebox'\n")
			var err error
			tempDir, err = os.MkdirTemp("", "gonic-jukebox-*")
			if err != nil {
				return fmt.Errorf("create tmp sock file: %w", err)
			}
			sockPath := filepath.Join(tempDir, "sock")
			if err := s.jukebox.Start(sockPath, mpvExtraArgs); err != nil {
				return fmt.Errorf("start jukebox: %w", err)
			}
			if err := s.jukebox.Wait(); err != nil {
				return fmt.Errorf("start jukebox: %w", err)
			}
			return nil
		}, func(_ error) {
			// stop job
			if err := s.jukebox.Quit(); err != nil {
				log.Printf("error quitting jukebox: %v", err)
			}
			_ = os.RemoveAll(tempDir)
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
				if err := s.podcast.RefreshPodcasts(); err != nil {
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

func (s *Server) StartPodcastPurger(maxAge time.Duration) (FuncExecute, FuncInterrupt) {
	ticker := time.NewTicker(24 * time.Hour)
	done := make(chan struct{})
	waitFor := func() error {
		for {
			select {
			case <-done:
				return nil
			case <-ticker.C:
				if err := s.podcast.PurgeOldPodcasts(maxAge); err != nil {
					log.Printf("error purging old podcasts: %v", err)
				}
			}
		}
	}
	return func() error {
			log.Printf("starting job 'podcast purger'\n")
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
