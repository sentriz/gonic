package server

func (s *Server) SetupSubsonic() error {
	withWare := newChain(
		s.WithLogging,
		s.WithCORS,
		s.WithValidSubsonicArgs,
	)
	// common
	s.mux.HandleFunc("/rest/download", withWare(s.Stream))
	s.mux.HandleFunc("/rest/download.view", withWare(s.Stream))
	s.mux.HandleFunc("/rest/stream", withWare(s.Stream))
	s.mux.HandleFunc("/rest/stream.view", withWare(s.Stream))
	s.mux.HandleFunc("/rest/getCoverArt", withWare(s.GetCoverArt))
	s.mux.HandleFunc("/rest/getCoverArt.view", withWare(s.GetCoverArt))
	s.mux.HandleFunc("/rest/getLicense", withWare(s.GetLicence))
	s.mux.HandleFunc("/rest/getLicense.view", withWare(s.GetLicence))
	s.mux.HandleFunc("/rest/ping", withWare(s.Ping))
	s.mux.HandleFunc("/rest/ping.view", withWare(s.Ping))
	s.mux.HandleFunc("/rest/scrobble", withWare(s.Scrobble))
	s.mux.HandleFunc("/rest/scrobble.view", withWare(s.Scrobble))
	s.mux.HandleFunc("/rest/getMusicFolders", withWare(s.GetMusicFolders))
	s.mux.HandleFunc("/rest/getMusicFolders.view", withWare(s.GetMusicFolders))
	s.mux.HandleFunc("/rest/startScan", withWare(s.StartScan))
	s.mux.HandleFunc("/rest/startScan.view", withWare(s.StartScan))
	s.mux.HandleFunc("/rest/getScanStatus", withWare(s.GetScanStatus))
	s.mux.HandleFunc("/rest/getScanStatus.view", withWare(s.GetScanStatus))
	s.mux.HandleFunc("/rest/getUser", withWare(s.GetUser))
	s.mux.HandleFunc("/rest/getUser.view", withWare(s.GetUser))
	s.mux.HandleFunc("/musicFolderSettings", withWare(s.MusicFolderSettings))
	s.mux.HandleFunc("/musicFolderSettings.view", withWare(s.MusicFolderSettings))
	// browse by tag
	s.mux.HandleFunc("/rest/getAlbum", withWare(s.GetAlbum))
	s.mux.HandleFunc("/rest/getAlbum.view", withWare(s.GetAlbum))
	s.mux.HandleFunc("/rest/getAlbumList2", withWare(s.GetAlbumListTwo))
	s.mux.HandleFunc("/rest/getAlbumList2.view", withWare(s.GetAlbumListTwo))
	s.mux.HandleFunc("/rest/getArtist", withWare(s.GetArtist))
	s.mux.HandleFunc("/rest/getArtist.view", withWare(s.GetArtist))
	s.mux.HandleFunc("/rest/getArtists", withWare(s.GetArtists))
	s.mux.HandleFunc("/rest/getArtists.view", withWare(s.GetArtists))
	s.mux.HandleFunc("/rest/search3", withWare(s.SearchThree))
	s.mux.HandleFunc("/rest/search3.view", withWare(s.SearchThree))
	// browse by folder
	s.mux.HandleFunc("/rest/getIndexes", withWare(s.GetIndexes))
	s.mux.HandleFunc("/rest/getIndexes.view", withWare(s.GetIndexes))
	s.mux.HandleFunc("/rest/getMusicDirectory", withWare(s.GetMusicDirectory))
	s.mux.HandleFunc("/rest/getMusicDirectory.view", withWare(s.GetMusicDirectory))
	s.mux.HandleFunc("/rest/getAlbumList", withWare(s.GetAlbumList))
	s.mux.HandleFunc("/rest/getAlbumList.view", withWare(s.GetAlbumList))
	s.mux.HandleFunc("/rest/search2", withWare(s.SearchTwo))
	s.mux.HandleFunc("/rest/search2.view", withWare(s.SearchTwo))
	return nil
}
