package ctrlsubsonic

import "github.com/gorilla/mux"

func AddRoutes(c *Controller, r *mux.Router) {
	r.Use(c.WithParams)
	r.Use(c.WithRequiredParams)
	r.Use(c.WithUser)

	// common
	r.Handle("/getLicense{_:(?:\\.view)?}", c.H(c.ServeGetLicence))
	r.Handle("/getMusicFolders{_:(?:\\.view)?}", c.H(c.ServeGetMusicFolders))
	r.Handle("/getScanStatus{_:(?:\\.view)?}", c.H(c.ServeGetScanStatus))
	r.Handle("/ping{_:(?:\\.view)?}", c.H(c.ServePing))
	r.Handle("/scrobble{_:(?:\\.view)?}", c.H(c.ServeScrobble))
	r.Handle("/startScan{_:(?:\\.view)?}", c.H(c.ServeStartScan))
	r.Handle("/getUser{_:(?:\\.view)?}", c.H(c.ServeGetUser))
	r.Handle("/getPlaylists{_:(?:\\.view)?}", c.H(c.ServeGetPlaylists))
	r.Handle("/getPlaylist{_:(?:\\.view)?}", c.H(c.ServeGetPlaylist))
	r.Handle("/createPlaylist{_:(?:\\.view)?}", c.H(c.ServeCreatePlaylist))
	r.Handle("/updatePlaylist{_:(?:\\.view)?}", c.H(c.ServeUpdatePlaylist))
	r.Handle("/deletePlaylist{_:(?:\\.view)?}", c.H(c.ServeDeletePlaylist))
	r.Handle("/savePlayQueue{_:(?:\\.view)?}", c.H(c.ServeSavePlayQueue))
	r.Handle("/getPlayQueue{_:(?:\\.view)?}", c.H(c.ServeGetPlayQueue))
	r.Handle("/getSong{_:(?:\\.view)?}", c.H(c.ServeGetSong))
	r.Handle("/getRandomSongs{_:(?:\\.view)?}", c.H(c.ServeGetRandomSongs))
	r.Handle("/getSongsByGenre{_:(?:\\.view)?}", c.H(c.ServeGetSongsByGenre))
	r.Handle("/jukeboxControl{_:(?:\\.view)?}", c.H(c.ServeJukebox))
	r.Handle("/getBookmarks{_:(?:\\.view)?}", c.H(c.ServeGetBookmarks))
	r.Handle("/createBookmark{_:(?:\\.view)?}", c.H(c.ServeCreateBookmark))
	r.Handle("/deleteBookmark{_:(?:\\.view)?}", c.H(c.ServeDeleteBookmark))
	r.Handle("/getTopSongs{_:(?:\\.view)?}", c.H(c.ServeGetTopSongs))
	r.Handle("/getSimilarSongs{_:(?:\\.view)?}", c.H(c.ServeGetSimilarSongs))
	r.Handle("/getSimilarSongs2{_:(?:\\.view)?}", c.H(c.ServeGetSimilarSongsTwo))
	r.Handle("/getLyrics{_:(?:\\.view)?}", c.H(c.ServeGetLyrics))

	// raw
	r.Handle("/getCoverArt{_:(?:\\.view)?}", c.HR(c.ServeGetCoverArt))
	r.Handle("/stream{_:(?:\\.view)?}", c.HR(c.ServeStream))
	r.Handle("/download{_:(?:\\.view)?}", c.HR(c.ServeStream))
	r.Handle("/getAvatar{_:(?:\\.view)?}", c.HR(c.ServeGetAvatar))

	// browse by tag
	r.Handle("/getAlbum{_:(?:\\.view)?}", c.H(c.ServeGetAlbum))
	r.Handle("/getAlbumList2{_:(?:\\.view)?}", c.H(c.ServeGetAlbumListTwo))
	r.Handle("/getArtist{_:(?:\\.view)?}", c.H(c.ServeGetArtist))
	r.Handle("/getArtists{_:(?:\\.view)?}", c.H(c.ServeGetArtists))
	r.Handle("/search3{_:(?:\\.view)?}", c.H(c.ServeSearchThree))
	r.Handle("/getArtistInfo2{_:(?:\\.view)?}", c.H(c.ServeGetArtistInfoTwo))
	r.Handle("/getStarred2{_:(?:\\.view)?}", c.H(c.ServeGetStarredTwo))

	// browse by folder
	r.Handle("/getIndexes{_:(?:\\.view)?}", c.H(c.ServeGetIndexes))
	r.Handle("/getMusicDirectory{_:(?:\\.view)?}", c.H(c.ServeGetMusicDirectory))
	r.Handle("/getAlbumList{_:(?:\\.view)?}", c.H(c.ServeGetAlbumList))
	r.Handle("/search2{_:(?:\\.view)?}", c.H(c.ServeSearchTwo))
	r.Handle("/getGenres{_:(?:\\.view)?}", c.H(c.ServeGetGenres))
	r.Handle("/getArtistInfo{_:(?:\\.view)?}", c.H(c.ServeGetArtistInfo))
	r.Handle("/getStarred{_:(?:\\.view)?}", c.H(c.ServeGetStarred))

	// star / rating
	r.Handle("/star{_:(?:\\.view)?}", c.H(c.ServeStar))
	r.Handle("/unstar{_:(?:\\.view)?}", c.H(c.ServeUnstar))
	r.Handle("/setRating{_:(?:\\.view)?}", c.H(c.ServeSetRating))

	// podcasts
	r.Handle("/getPodcasts{_:(?:\\.view)?}", c.H(c.ServeGetPodcasts))
	r.Handle("/getNewestPodcasts{_:(?:\\.view)?}", c.H(c.ServeGetNewestPodcasts))
	r.Handle("/downloadPodcastEpisode{_:(?:\\.view)?}", c.H(c.ServeDownloadPodcastEpisode))
	r.Handle("/createPodcastChannel{_:(?:\\.view)?}", c.H(c.ServeCreatePodcastChannel))
	r.Handle("/refreshPodcasts{_:(?:\\.view)?}", c.H(c.ServeRefreshPodcasts))
	r.Handle("/deletePodcastChannel{_:(?:\\.view)?}", c.H(c.ServeDeletePodcastChannel))
	r.Handle("/deletePodcastEpisode{_:(?:\\.view)?}", c.H(c.ServeDeletePodcastEpisode))

	// internet radio
	r.Handle("/getInternetRadioStations{_:(?:\\.view)?}", c.H(c.ServeGetInternetRadioStations))
	r.Handle("/createInternetRadioStation{_:(?:\\.view)?}", c.H(c.ServeCreateInternetRadioStation))
	r.Handle("/updateInternetRadioStation{_:(?:\\.view)?}", c.H(c.ServeUpdateInternetRadioStation))
	r.Handle("/deleteInternetRadioStation{_:(?:\\.view)?}", c.H(c.ServeDeleteInternetRadioStation))

	// middlewares should be run for not found handler
	// https://github.com/gorilla/mux/issues/416
	notFoundHandler := c.H(c.ServeNotFound)
	notFoundRoute := r.NewRoute().Handler(notFoundHandler)
	r.NotFoundHandler = notFoundRoute.GetHandler()
}
