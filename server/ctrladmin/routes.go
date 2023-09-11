package ctrladmin

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.senan.xyz/gonic/server/ctrladmin/adminui"
)

func AddRoutes(c *Controller, r *mux.Router) {
	// public routes (creates session)
	r.Use(c.WithSession)
	r.Handle("/login", c.H(c.ServeLogin))
	r.Handle("/login_do", c.HR(c.ServeLoginDo)) // "raw" handler, updates session

	staticHandler := http.StripPrefix("/admin", http.FileServer(http.FS(adminui.StaticFS)))
	r.PathPrefix("/static").Handler(staticHandler)

	// user routes (if session is valid)
	routUser := r.NewRoute().Subrouter()
	routUser.Use(c.WithUserSession)
	routUser.Handle("/logout", c.HR(c.ServeLogout)) // "raw" handler, updates session
	routUser.Handle("/home", c.H(c.ServeHome))
	routUser.Handle("/change_username", c.H(c.ServeChangeUsername))
	routUser.Handle("/change_username_do", c.H(c.ServeChangeUsernameDo))
	routUser.Handle("/change_password", c.H(c.ServeChangePassword))
	routUser.Handle("/change_password_do", c.H(c.ServeChangePasswordDo))
	routUser.Handle("/change_avatar", c.H(c.ServeChangeAvatar))
	routUser.Handle("/change_avatar_do", c.H(c.ServeChangeAvatarDo))
	routUser.Handle("/delete_avatar_do", c.H(c.ServeDeleteAvatarDo))
	routUser.Handle("/delete_user", c.H(c.ServeDeleteUser))
	routUser.Handle("/delete_user_do", c.H(c.ServeDeleteUserDo))
	routUser.Handle("/link_lastfm_do", c.H(c.ServeLinkLastFMDo))
	routUser.Handle("/unlink_lastfm_do", c.H(c.ServeUnlinkLastFMDo))
	routUser.Handle("/link_listenbrainz_do", c.H(c.ServeLinkListenBrainzDo))
	routUser.Handle("/unlink_listenbrainz_do", c.H(c.ServeUnlinkListenBrainzDo))
	routUser.Handle("/create_transcode_pref_do", c.H(c.ServeCreateTranscodePrefDo))
	routUser.Handle("/delete_transcode_pref_do", c.H(c.ServeDeleteTranscodePrefDo))

	// admin routes (if session is valid, and is admin)
	routAdmin := routUser.NewRoute().Subrouter()
	routAdmin.Use(c.WithAdminSession)
	routAdmin.Handle("/create_user", c.H(c.ServeCreateUser))
	routAdmin.Handle("/create_user_do", c.H(c.ServeCreateUserDo))
	routAdmin.Handle("/update_lastfm_api_key", c.H(c.ServeUpdateLastFMAPIKey))
	routAdmin.Handle("/update_lastfm_api_key_do", c.H(c.ServeUpdateLastFMAPIKeyDo))
	routAdmin.Handle("/start_scan_inc_do", c.H(c.ServeStartScanIncDo))
	routAdmin.Handle("/start_scan_full_do", c.H(c.ServeStartScanFullDo))
	routAdmin.Handle("/add_podcast_do", c.H(c.ServePodcastAddDo))
	routAdmin.Handle("/delete_podcast_do", c.H(c.ServePodcastDeleteDo))
	routAdmin.Handle("/download_podcast_do", c.H(c.ServePodcastDownloadDo))
	routAdmin.Handle("/update_podcast_do", c.H(c.ServePodcastUpdateDo))
	routAdmin.Handle("/add_internet_radio_station_do", c.H(c.ServeInternetRadioStationAddDo))
	routAdmin.Handle("/delete_internet_radio_station_do", c.H(c.ServeInternetRadioStationDeleteDo))
	routAdmin.Handle("/update_internet_radio_station_do", c.H(c.ServeInternetRadioStationUpdateDo))

	// middlewares should be run for not found handler
	// https://github.com/gorilla/mux/issues/416
	notFoundHandler := c.H(c.ServeNotFound)
	notFoundRoute := r.NewRoute().Handler(notFoundHandler)
	r.NotFoundHandler = notFoundRoute.GetHandler()
}
