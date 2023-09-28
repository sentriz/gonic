//nolint:lll,gocyclo,forbidigo
package main

import (
	"errors"
	"expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/shlex"
	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff"
	"github.com/sentriz/gormstore"

	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/jukebox"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/listenbrainz"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/podcasts"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/scanner/tags"
	"go.senan.xyz/gonic/scrobble"
	"go.senan.xyz/gonic/server/ctrladmin"
	"go.senan.xyz/gonic/server/ctrlbase"
	"go.senan.xyz/gonic/server/ctrlsubsonic"
	"go.senan.xyz/gonic/server/ctrlsubsonic/artistinfocache"
	"go.senan.xyz/gonic/transcode"
)

func main() {
	set := flag.NewFlagSet(gonic.Name, flag.ExitOnError)
	confListenAddr := set.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")

	confTLSCert := set.String("tls-cert", "", "path to TLS certificate (optional)")
	confTLSKey := set.String("tls-key", "", "path to TLS private key (optional)")

	confPodcastPurgeAgeDays := set.Int("podcast-purge-age", 0, "age (in days) to purge podcast episodes if not accessed (optional)")
	confPodcastPath := set.String("podcast-path", "", "path to podcasts")

	confCachePath := set.String("cache-path", "", "path to cache")

	var confMusicPaths pathAliases
	set.Var(&confMusicPaths, "music-path", "path to music")

	confPlaylistsPath := set.String("playlists-path", "", "path to your list of new or existing m3u playlists that gonic can manage")

	confDBPath := set.String("db-path", "gonic.db", "path to database (optional)")

	confScanIntervalMins := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	confScanAtStart := set.Bool("scan-at-start-enabled", false, "whether to perform an initial scan at startup (optional)")
	confScanWatcher := set.Bool("scan-watcher-enabled", false, "whether to watch file system for new music and rescan (optional)")

	confJukeboxEnabled := set.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
	confJukeboxMPVExtraArgs := set.String("jukebox-mpv-extra-args", "", "extra command line arguments to pass to the jukebox mpv daemon (optional)")

	confProxyPrefix := set.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	confHTTPLog := set.Bool("http-log", true, "http request logging (optional)")

	confShowVersion := set.Bool("version", false, "show gonic version")
	_ = set.String("config-path", "", "path to config (optional)")

	confExcludePatterns := set.String("exclude-pattern", "", "regex pattern to exclude files from scan (optional)")

	var confMultiValueGenre, confMultiValueAlbumArtist multiValueSetting
	set.Var(&confMultiValueGenre, "multi-value-genre", "setting for mutli-valued genre scanning (optional)")
	set.Var(&confMultiValueAlbumArtist, "multi-value-album-artist", "setting for mutli-valued album artist scanning (optional)")

	confExpvar := set.Bool("expvar", false, "enable the /debug/vars endpoint (optional)")

	deprecatedConfGenreSplit := set.String("genre-split", "", "(deprecated, see multi-value settings)")

	if _, err := regexp.Compile(*confExcludePatterns); err != nil {
		log.Fatalf("invalid exclude pattern: %v\n", err)
	}

	if err := ff.Parse(set, os.Args[1:],
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix(gonic.NameUpper),
	); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}

	if *confShowVersion {
		fmt.Printf("v%s\n", gonic.Version)
		os.Exit(0)
	}

	if len(confMusicPaths) == 0 {
		log.Fatalf("please provide a music directory")
	}

	var err error
	for i, confMusicPath := range confMusicPaths {
		if confMusicPaths[i].path, err = validatePath(confMusicPath.path); err != nil {
			log.Fatalf("checking music dir %q: %v", confMusicPath.path, err)
		}
	}

	if *confPodcastPath, err = validatePath(*confPodcastPath); err != nil {
		log.Fatalf("checking podcast directory: %v", err)
	}
	if *confCachePath, err = validatePath(*confCachePath); err != nil {
		log.Fatalf("checking cache directory: %v", err)
	}
	if *confPlaylistsPath, err = validatePath(*confPlaylistsPath); err != nil {
		log.Fatalf("checking playlist directory: %v", err)
	}

	cacheDirAudio := path.Join(*confCachePath, "audio")
	cacheDirCovers := path.Join(*confCachePath, "covers")
	if err := os.MkdirAll(cacheDirAudio, os.ModePerm); err != nil {
		log.Fatalf("couldn't create audio cache path: %v\n", err)
	}
	if err := os.MkdirAll(cacheDirCovers, os.ModePerm); err != nil {
		log.Fatalf("couldn't create covers cache path: %v\n", err)
	}

	dbc, err := db.New(*confDBPath, db.DefaultOptions())
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer dbc.Close()

	err = dbc.Migrate(db.MigrationContext{
		OriginalMusicPath: confMusicPaths[0].path,
		PlaylistsPath:     *confPlaylistsPath,
		PodcastsPath:      *confPodcastPath,
	})
	if err != nil {
		log.Panicf("error migrating database: %v\n", err)
	}

	var musicPaths []ctrlsubsonic.MusicPath
	for _, pa := range confMusicPaths {
		musicPaths = append(musicPaths, ctrlsubsonic.MusicPath{Alias: pa.alias, Path: pa.path})
	}

	proxyPrefixExpr := regexp.MustCompile(`^\/*(.*?)\/*$`)
	*confProxyPrefix = proxyPrefixExpr.ReplaceAllString(*confProxyPrefix, `/$1`)

	if *deprecatedConfGenreSplit != "" && *deprecatedConfGenreSplit != "\n" {
		confMultiValueGenre = multiValueSetting{Mode: scanner.Delim, Delim: *deprecatedConfGenreSplit}
		*deprecatedConfGenreSplit = "<deprecated>"
	}

	log.Printf("starting gonic v%s\n", gonic.Version)
	log.Printf("provided config\n")
	set.VisitAll(func(f *flag.Flag) {
		value := strings.ReplaceAll(f.Value.String(), "\n", "")
		log.Printf("    %-25s %s\n", f.Name, value)
	})

	tagger := &tags.TagReader{}
	scannr := scanner.New(
		ctrlsubsonic.PathsOf(musicPaths),
		dbc,
		map[scanner.Tag]scanner.MultiValueSetting{
			scanner.Genre:       scanner.MultiValueSetting(confMultiValueGenre),
			scanner.AlbumArtist: scanner.MultiValueSetting(confMultiValueAlbumArtist),
		},
		tagger,
		*confExcludePatterns,
	)
	podcast := podcasts.New(dbc, *confPodcastPath, tagger)
	transcoder := transcode.NewCachingTranscoder(
		transcode.NewFFmpegTranscoder(),
		cacheDirAudio,
	)

	lastfmClientKeySecretFunc := func() (string, string, error) {
		apiKey, _ := dbc.GetSetting(db.LastFMAPIKey)
		secret, _ := dbc.GetSetting(db.LastFMSecret)
		if apiKey == "" || secret == "" {
			return "", "", fmt.Errorf("not configured")
		}
		return apiKey, secret, nil
	}

	listenbrainzClient := listenbrainz.NewClient()
	lastfmClient := lastfm.NewClient(lastfmClientKeySecretFunc)

	playlistStore, err := playlist.NewStore(*confPlaylistsPath)
	if err != nil {
		log.Panicf("error creating playlists store: %v", err)
	}

	var jukebx *jukebox.Jukebox
	if *confJukeboxEnabled {
		jukebx = jukebox.New()
	}

	sessKey, err := dbc.GetSetting("session_key")
	if err != nil {
		log.Panicf("error getting session key: %v\n", err)
	}
	if sessKey == "" {
		if err := dbc.SetSetting("session_key", string(securecookie.GenerateRandomKey(32))); err != nil {
			log.Panicf("error setting session key: %v\n", err)
		}
	}
	sessDB := gormstore.New(dbc.DB, []byte(sessKey))
	sessDB.SessionOpts.HttpOnly = true
	sessDB.SessionOpts.SameSite = http.SameSiteLaxMode

	artistInfoCache := artistinfocache.New(dbc, lastfmClient)

	ctrlBase := &ctrlbase.Controller{
		DB:            dbc,
		PlaylistStore: playlistStore,
		ProxyPrefix:   *confProxyPrefix,
		Scanner:       scannr,
	}
	ctrlAdmin, err := ctrladmin.New(ctrlBase, sessDB, podcast, lastfmClient)
	if err != nil {
		log.Panicf("error creating admin controller: %v\n", err)
	}
	ctrlSubsonic := &ctrlsubsonic.Controller{
		Controller:      ctrlBase,
		MusicPaths:      musicPaths,
		PodcastsPath:    *confPodcastPath,
		CacheAudioPath:  cacheDirAudio,
		CacheCoverPath:  cacheDirCovers,
		LastFMClient:    lastfmClient,
		ArtistInfoCache: artistInfoCache,
		Scrobblers: []scrobble.Scrobbler{
			lastfmClient,
			listenbrainzClient,
		},
		Podcasts:   podcast,
		Transcoder: transcoder,
		Jukebox:    jukebx,
	}

	mux := mux.NewRouter()
	ctrlbase.AddRoutes(ctrlBase, mux, *confHTTPLog)
	ctrladmin.AddRoutes(ctrlAdmin, mux.PathPrefix("/admin").Subrouter())
	ctrlsubsonic.AddRoutes(ctrlSubsonic, mux.PathPrefix("/rest").Subrouter())

	if *confExpvar {
		mux.Handle("/debug/vars", expvar.Handler())
		expvar.Publish("stats", expvar.Func(func() any {
			var stats struct{ Albums, Tracks, Artists, InternetRadioStations, Podcasts uint }
			dbc.Model(db.Album{}).Count(&stats.Albums)
			dbc.Model(db.Track{}).Count(&stats.Tracks)
			dbc.Model(db.Artist{}).Count(&stats.Artists)
			dbc.Model(db.InternetRadioStation{}).Count(&stats.InternetRadioStations)
			dbc.Model(db.Podcast{}).Count(&stats.Podcasts)
			return stats
		}))
	}

	noCleanup := func(_ error) {}

	var g run.Group
	g.Add(func() error {
		log.Print("starting job 'http'\n")
		server := &http.Server{
			Addr:              *confListenAddr,
			Handler:           mux,
			ReadTimeout:       5 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      80 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if *confTLSCert != "" && *confTLSKey != "" {
			return server.ListenAndServeTLS(*confTLSCert, *confTLSKey)
		}
		return server.ListenAndServe()
	}, noCleanup)

	g.Add(func() error {
		log.Printf("starting job 'session clean'\n")
		ticker := time.NewTicker(10 * time.Minute)
		for range ticker.C {
			sessDB.Cleanup()
		}
		return nil
	}, noCleanup)

	g.Add(func() error {
		log.Printf("starting job 'podcast refresher'\n")
		ticker := time.NewTicker(time.Hour)
		for range ticker.C {
			if err := podcast.RefreshPodcasts(); err != nil {
				log.Printf("failed to refresh some feeds: %s", err)
			}
		}
		return nil
	}, noCleanup)

	if *confPodcastPurgeAgeDays > 0 {
		g.Add(func() error {
			log.Printf("starting job 'podcast purger'\n")
			ticker := time.NewTicker(24 * time.Hour)
			for range ticker.C {
				if err := podcast.PurgeOldPodcasts(time.Duration(*confPodcastPurgeAgeDays) * 24 * time.Hour); err != nil {
					log.Printf("error purging old podcasts: %v", err)
				}
			}
			return nil
		}, noCleanup)
	}

	if *confScanIntervalMins > 0 {
		g.Add(func() error {
			log.Printf("starting job 'scan timer'\n")
			ticker := time.NewTicker(time.Duration(*confScanIntervalMins) * time.Minute)
			for range ticker.C {
				if _, err := scannr.ScanAndClean(scanner.ScanOptions{}); err != nil {
					log.Printf("error scanning: %v", err)
				}
			}
			return nil
		}, noCleanup)
	}

	if *confScanWatcher {
		g.Add(func() error {
			log.Printf("starting job 'scan watcher'\n")
			return scannr.ExecuteWatch()
		}, func(_ error) {
			scannr.CancelWatch()
		})
	}

	if jukebx != nil {
		var jukeboxTempDir string
		g.Add(func() error {
			log.Printf("starting job 'jukebox'\n")
			extraArgs, _ := shlex.Split(*confJukeboxMPVExtraArgs)
			var err error
			jukeboxTempDir, err = os.MkdirTemp("", "gonic-jukebox-*")
			if err != nil {
				return fmt.Errorf("create tmp sock file: %w", err)
			}
			sockPath := filepath.Join(jukeboxTempDir, "sock")
			if err := jukebx.Start(sockPath, extraArgs); err != nil {
				return fmt.Errorf("start jukebox: %w", err)
			}
			if err := jukebx.Wait(); err != nil {
				return fmt.Errorf("start jukebox: %w", err)
			}
			return nil
		}, func(_ error) {
			if err := jukebx.Quit(); err != nil {
				log.Printf("error quitting jukebox: %v", err)
			}
			_ = os.RemoveAll(jukeboxTempDir)
		})
	}

	if _, _, err := lastfmClientKeySecretFunc(); err == nil {
		g.Add(func() error {
			log.Printf("starting job 'refresh artist info'\n")
			return artistInfoCache.Refresh(8 * time.Second)
		}, noCleanup)
	}

	if *confScanAtStart {
		if _, err := scannr.ScanAndClean(scanner.ScanOptions{}); err != nil {
			log.Panicf("error scanning at start: %v\n", err)
		}
	}

	if err := g.Run(); err != nil {
		log.Panicf("error in job: %v", err)
	}
}

const pathAliasSep = "->"

type (
	pathAliases []pathAlias
	pathAlias   struct{ alias, path string }
)

func (pa pathAliases) String() string {
	var strs []string
	for _, p := range pa {
		if p.alias != "" {
			strs = append(strs, fmt.Sprintf("%s %s %s", p.alias, pathAliasSep, p.path))
			continue
		}
		strs = append(strs, p.path)
	}
	return strings.Join(strs, ", ")
}

func (pa *pathAliases) Set(value string) error {
	if name, path, ok := strings.Cut(value, pathAliasSep); ok {
		*pa = append(*pa, pathAlias{alias: name, path: path})
		return nil
	}
	*pa = append(*pa, pathAlias{path: value})
	return nil
}

func validatePath(p string) (string, error) {
	if p == "" {
		return "", errors.New("path can't be empty")
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return "", errors.New("path does not exist, please provide one")
	}
	p, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("make absolute: %w", err)
	}
	return p, nil
}

type multiValueSetting scanner.MultiValueSetting

func (mvs multiValueSetting) String() string {
	switch mvs.Mode {
	case scanner.Delim:
		return fmt.Sprintf("delim(%s)", mvs.Delim)
	case scanner.Multi:
		return fmt.Sprint("multi", mvs.Delim)
	default:
		return "none"
	}
}

func (mvs *multiValueSetting) Set(value string) error {
	mode, delim, _ := strings.Cut(value, " ")
	switch mode {
	case "delim":
		if delim == "" {
			return fmt.Errorf("no delimiter provided for delimiter mode")
		}
		mvs.Mode = scanner.Delim
		mvs.Delim = delim
	case "multi":
		mvs.Mode = scanner.Multi
	case "none":
	default:
		return fmt.Errorf(`unknown multi value mode %q. should be "none" | "multi" | "delim <delim>"`, mode)
	}
	return nil
}
