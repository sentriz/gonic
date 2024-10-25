//nolint:lll,gocyclo,forbidigo,nilerr,errcheck
package main

import (
	"context"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	// avatar encode/decode
	_ "image/gif"
	_ "image/png"

	"github.com/google/shlex"
	"github.com/gorilla/securecookie"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/sentriz/gormstore"
	"golang.org/x/sync/errgroup"

	"go.senan.xyz/flagconf"
	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/handlerutil"
	"go.senan.xyz/gonic/infocache/albuminfocache"
	"go.senan.xyz/gonic/infocache/artistinfocache"
	"go.senan.xyz/gonic/jukebox"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/listenbrainz"
	"go.senan.xyz/gonic/playlist"
	"go.senan.xyz/gonic/podcast"
	"go.senan.xyz/gonic/sandbox"
	"go.senan.xyz/gonic/scanner"
	"go.senan.xyz/gonic/scrobble"
	"go.senan.xyz/gonic/server/ctrladmin"
	"go.senan.xyz/gonic/server/ctrlsubsonic"
	"go.senan.xyz/gonic/tags/tagcommon"
	"go.senan.xyz/gonic/tags/taglib"
	"go.senan.xyz/gonic/transcode"
)

func main() {
	sandbox.Init()
	confListenAddr := flag.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")

	confTLSCert := flag.String("tls-cert", "", "path to TLS certificate (optional)")
	confTLSKey := flag.String("tls-key", "", "path to TLS private key (optional)")

	confPodcastPurgeAgeDays := flag.Uint("podcast-purge-age", 0, "age (in days) to purge podcast episodes if not accessed (optional)")
	confPodcastPath := flag.String("podcast-path", "", "path to podcasts")

	confCachePath := flag.String("cache-path", "", "path to cache")

	var confMusicPaths pathAliases
	flag.Var(&confMusicPaths, "music-path", "path to music")

	confPlaylistsPath := flag.String("playlists-path", "", "path to your list of new or existing m3u playlists that gonic can manage")

	confDBPath := flag.String("db-path", "gonic.db", "path to database (optional)")

	confScanIntervalMins := flag.Uint("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	confScanAtStart := flag.Bool("scan-at-start-enabled", false, "whether to perform an initial scan at startup (optional)")
	confScanWatcher := flag.Bool("scan-watcher-enabled", false, "whether to watch file system for new music and rescan (optional)")

	confJukeboxEnabled := flag.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
	confJukeboxMPVExtraArgs := flag.String("jukebox-mpv-extra-args", "", "extra command line arguments to pass to the jukebox mpv daemon (optional)")

	confProxyPrefix := flag.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	confHTTPLog := flag.Bool("http-log", true, "http request logging (optional)")

	confShowVersion := flag.Bool("version", false, "show gonic version")
	confConfigPath := flag.String("config-path", "", "path to config (optional)")

	confExcludePattern := flag.String("exclude-pattern", "", "regex pattern to exclude files from scan (optional)")

	var confMultiValueGenre, confMultiValueArtist, confMultiValueAlbumArtist multiValueSetting
	flag.Var(&confMultiValueGenre, "multi-value-genre", "setting for mutli-valued genre scanning (optional)")
	flag.Var(&confMultiValueArtist, "multi-value-artist", "setting for mutli-valued track artist scanning (optional)")
	flag.Var(&confMultiValueAlbumArtist, "multi-value-album-artist", "setting for mutli-valued album artist scanning (optional)")

	confPprof := flag.Bool("pprof", false, "enable the /debug/pprof endpoint (optional)")
	confExpvar := flag.Bool("expvar", false, "enable the /debug/vars endpoint (optional)")

	deprecatedConfGenreSplit := flag.String("genre-split", "", "(deprecated, see multi-value settings)")

	confTranscodeCacheSize := flag.Int("transcode-cache-size", 0, "size of the transcode cache in MB (0 = no limit) (optional)")
	confTranscodeEjectInterval := flag.Int("transcode-eject-interval", 0, "interval (in minutes) to eject transcode cache (0 = never) (optional)")

	flag.Parse()
	flagconf.ParseEnv()

	if *confConfigPath != "" {
		sandbox.ReadOnlyPath(*confConfigPath)
		flagconf.ParseConfig(*confConfigPath)
	}

	if *confShowVersion {
		fmt.Printf("v%s\n", gonic.Version)
		os.Exit(0)
	}

	if _, err := regexp.Compile(*confExcludePattern); err != nil {
		log.Fatalf("invalid exclude pattern: %v\n", err)
	}

	if len(confMusicPaths) == 0 {
		log.Fatalf("please provide a music directory")
	}

	var err error
	for i, confMusicPath := range confMusicPaths {
		sandbox.ReadOnlyPath(confMusicPath.path)
		if confMusicPaths[i].path, err = validatePath(confMusicPath.path); err != nil {
			log.Fatalf("checking music dir %q: %v", confMusicPath.path, err)
		}
	}

	sandbox.ReadWriteCreatePath(*confPodcastPath)
	if *confPodcastPath, err = validatePath(*confPodcastPath); err != nil {
		log.Fatalf("checking podcast directory: %v", err)
	}
	sandbox.ReadWriteCreatePath(*confCachePath)
	if *confCachePath, err = validatePath(*confCachePath); err != nil {
		log.Fatalf("checking cache directory: %v", err)
	}
	sandbox.ReadWriteCreatePath(*confPlaylistsPath)
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
		Production:        true,
		DBPath:            *confDBPath,
		OriginalMusicPath: confMusicPaths[0].path,
		PlaylistsPath:     *confPlaylistsPath,
		PodcastsPath:      *confPodcastPath,
	})
	if err != nil {
		log.Panicf("error migrating database: %v\n", err)
	}

	if *confTLSCert != "" && *confTLSKey != "" {
		sandbox.ReadOnlyPath(*confTLSCert)
		sandbox.ReadOnlyPath(*confTLSKey)
	}

	sandbox.AllPathsAdded()

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
	if confMultiValueArtist.Mode == scanner.None && confMultiValueAlbumArtist.Mode > scanner.None {
		confMultiValueArtist.Mode = confMultiValueAlbumArtist.Mode
		confMultiValueArtist.Delim = confMultiValueAlbumArtist.Delim
	}
	if confMultiValueArtist.Mode != confMultiValueAlbumArtist.Mode {
		log.Panic("differing multi artist and album artist modes have been tested yet. please set them to be the same")
	}

	log.Printf("starting gonic v%s\n", gonic.Version)
	log.Printf("provided config\n")
	flag.VisitAll(func(f *flag.Flag) {
		value := strings.ReplaceAll(f.Value.String(), "\n", "")
		log.Printf("    %-25s %s\n", f.Name, value)
	})

	tagReader := tagcommon.ChainReader{
		taglib.TagLib{},
		// ffprobe reader?
		// nfo reader?
	}

	scannr := scanner.New(
		ctrlsubsonic.MusicPaths(musicPaths),
		dbc,
		map[scanner.Tag]scanner.MultiValueSetting{
			scanner.Genre:       scanner.MultiValueSetting(confMultiValueGenre),
			scanner.Artist:      scanner.MultiValueSetting(confMultiValueArtist),
			scanner.AlbumArtist: scanner.MultiValueSetting(confMultiValueAlbumArtist),
		},
		tagReader,
		*confExcludePattern,
	)
	podcast := podcast.New(dbc, *confPodcastPath, tagReader)
	transcoder := transcode.NewCachingTranscoder(
		transcode.NewFFmpegTranscoder(),
		cacheDirAudio,
		*confTranscodeCacheSize,
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
		sessKey = string(securecookie.GenerateRandomKey(32))
		if err := dbc.SetSetting("session_key", sessKey); err != nil {
			log.Panicf("error setting session key: %v\n", err)
		}
	}
	sessDB := gormstore.New(dbc.DB, []byte(sessKey))
	sessDB.SessionOpts.HttpOnly = true
	sessDB.SessionOpts.SameSite = http.SameSiteLaxMode

	artistInfoCache := artistinfocache.New(dbc, lastfmClient)
	albumInfoCache := albuminfocache.New(dbc, lastfmClient)

	scrobblers := []scrobble.Scrobbler{lastfmClient, listenbrainzClient}

	resolveProxyPath := func(in string) string {
		url, _ := url.Parse(in)
		url.Path = path.Join(*confProxyPrefix, url.Path)
		return url.String()
	}

	ctrlAdmin, err := ctrladmin.New(dbc, sessDB, scannr, podcast, lastfmClient, resolveProxyPath)
	if err != nil {
		log.Panicf("error creating admin controller: %v\n", err)
	}
	ctrlSubsonic, err := ctrlsubsonic.New(dbc, scannr, musicPaths, *confPodcastPath, cacheDirAudio, cacheDirCovers, jukebx, playlistStore, scrobblers, podcast, transcoder, lastfmClient, artistInfoCache, albumInfoCache, resolveProxyPath)
	if err != nil {
		log.Panicf("error creating subsonic controller: %v\n", err)
	}

	chain := handlerutil.Chain()
	if *confHTTPLog {
		chain = handlerutil.Chain(handlerutil.Log)
	}
	chain = handlerutil.Chain(
		chain,
		handlerutil.BasicCORS,
	)
	trim := handlerutil.TrimPathSuffix(".view") // /x.view and /x should match the same

	mux := http.NewServeMux()
	mux.Handle("/admin/", http.StripPrefix("/admin", chain(ctrlAdmin)))
	mux.Handle("/rest/", http.StripPrefix("/rest", chain(trim(ctrlSubsonic))))
	mux.Handle("/ping", chain(handlerutil.Message("ok")))
	mux.Handle("/", chain(http.RedirectHandler(resolveProxyPath("/admin/home"), http.StatusSeeOther)))

	if *confExpvar {
		mux.Handle("/debug/vars", expvar.Handler())
		expvar.Publish("stats", expvar.Func(func() any {
			stats, _ := dbc.Stats()
			return stats
		}))
	}

	var (
		readTimeout  = 5 * time.Second
		writeTimeout = 5 * time.Second
		idleTimeout  = 5 * time.Second
	)

	if *confPprof {
		// overwrite global WriteTimeout. in future we should set this only for these handlers
		// https://github.com/golang/go/issues/62358
		writeTimeout = 0

		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		defer logJob("http")()

		server := &http.Server{
			Addr:        *confListenAddr,
			ReadTimeout: readTimeout, WriteTimeout: writeTimeout, IdleTimeout: idleTimeout,
			Handler: mux,
		}
		errgrp.Go(func() error {
			<-ctx.Done()
			return server.Shutdown(context.Background())
		})
		if *confTLSCert != "" && *confTLSKey != "" {
			return server.ListenAndServeTLS(*confTLSCert, *confTLSKey)
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	errgrp.Go(func() error {
		if !*confScanWatcher {
			return nil
		}

		defer logJob("scan watcher")()

		return scannr.ExecuteWatch(ctx)
	})

	errgrp.Go(func() error {
		if jukebx == nil {
			return nil
		}

		defer logJob("jukebox")()

		extraArgs, _ := shlex.Split(*confJukeboxMPVExtraArgs)
		jukeboxTempDir := filepath.Join(*confCachePath, "gonic-jukebox")
		if err := os.RemoveAll(jukeboxTempDir); err != nil {
			return fmt.Errorf("remove jubebox tmp dir: %w", err)
		}
		if err := os.MkdirAll(jukeboxTempDir, os.ModePerm); err != nil {
			return fmt.Errorf("create tmp sock file: %w", err)
		}
		sockPath := filepath.Join(jukeboxTempDir, "sock")
		if err := jukebx.Start(ctx, sockPath, extraArgs); err != nil {
			return fmt.Errorf("start jukebox: %w", err)
		}
		return nil
	})

	errgrp.Go(func() error {
		defer logJob("session clean")()

		ctxTick(ctx, 10*time.Minute, func() {
			sessDB.Cleanup()
		})
		return nil
	})

	errgrp.Go(func() error {
		defer logJob("podcast refresh")()

		ctxTick(ctx, 1*time.Hour, func() {
			if err := podcast.RefreshPodcasts(); err != nil {
				log.Printf("failed to refresh some feeds: %s", err)
			}
		})
		return nil
	})

	errgrp.Go(func() error {
		defer logJob("podcast download")()

		ctxTick(ctx, 5*time.Second, func() {
			if err := podcast.DownloadTick(); err != nil {
				log.Printf("failed to download podcast: %s", err)
			}
		})
		return nil
	})

	errgrp.Go(func() error {
		if *confPodcastPurgeAgeDays == 0 {
			return nil
		}

		defer logJob("podcast purge")()

		ctxTick(ctx, 24*time.Hour, func() {
			if err := podcast.PurgeOldPodcasts(time.Duration(*confPodcastPurgeAgeDays) * 24 * time.Hour); err != nil {
				log.Printf("error purging old podcasts: %v", err)
			}
		})
		return nil
	})

	errgrp.Go(func() error {
		if *confTranscodeEjectInterval == 0 || *confTranscodeCacheSize == 0 {
			return nil
		}

		defer logJob("transcode cache eject")()

		ctxTick(ctx, time.Duration(*confTranscodeEjectInterval)*time.Minute, func() {
			if err := transcoder.CacheEject(); err != nil {
				log.Printf("error ejecting transcode cache: %v", err)
			}
		})
		return nil
	})

	errgrp.Go(func() error {
		if *confScanIntervalMins == 0 {
			return nil
		}

		defer logJob("scan timer")()

		ctxTick(ctx, time.Duration(*confScanIntervalMins)*time.Minute, func() {
			if _, err := scannr.ScanAndClean(scanner.ScanOptions{}); err != nil {
				log.Printf("error scanning: %v", err)
			}
		})
		return nil
	})

	errgrp.Go(func() error {
		if _, _, err := lastfmClientKeySecretFunc(); err != nil {
			return nil
		}

		defer logJob("refresh artist info")()

		ctxTick(ctx, 8*time.Second, func() {
			if err := artistInfoCache.Refresh(); err != nil {
				log.Printf("error in artist info cache: %v", err)
			}
		})
		return nil
	})

	errgrp.Go(func() error {
		if !*confScanAtStart {
			return nil
		}

		defer logJob("scan at start")()

		if _, err := scannr.ScanAndClean(scanner.ScanOptions{}); err != nil {
			log.Printf("error scanning on start: %v", err)
		}
		return nil
	})

	if err := errgrp.Wait(); err != nil {
		log.Panic(err)
	}

	fmt.Println("shutdown complete")
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
		return "multi"
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

func logJob(jobName string) func() {
	log.Printf("starting job %q", jobName)
	return func() { log.Printf("stopped job %q", jobName) }
}

func ctxTick(ctx context.Context, interval time.Duration, f func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			f()
		}
	}
}
