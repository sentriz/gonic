// Package main is the gonic server entrypoint
//
//nolint:lll // flags help strings
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/shlex"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff"

	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/paths"
	"go.senan.xyz/gonic/server"
)

const (
	cleanTimeDuration = 10 * time.Minute
	cachePrefixAudio  = "audio"
	cachePrefixCovers = "covers"
)

func main() {
	set := flag.NewFlagSet(gonic.Name, flag.ExitOnError)
	confListenAddr := set.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")
	confTLSCert := set.String("tls-cert", "", "path to TLS certificate (optional)")
	confTLSKey := set.String("tls-key", "", "path to TLS private key (optional)")
	confPodcastPath := set.String("podcast-path", "", "path to podcasts")
	confCachePath := set.String("cache-path", "", "path to cache")
	confDBPath := set.String("db-path", "gonic.db", "path to database (optional)")
	confPlaylistPath := set.String("playlist-path", "", "path to directory containing m3u playlist files")
	confScanIntervalMins := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	confScanAtStart := set.Bool("scan-at-start-enabled", false, "whether to perform an initial scan at startup (optional)")
	confScanWatcher := set.Bool("scan-watcher-enabled", false, "whether to watch file system for new music and rescan (optional)")
	confJukeboxEnabled := set.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
	confJukeboxMPVExtraArgs := set.String("jukebox-mpv-extra-args", "", "extra command line arguments to pass to the jukebox mpv daemon (optional)")
	confPodcastPurgeAgeDays := set.Int("podcast-purge-age", 0, "age (in days) to purge podcast episodes if not accessed (optional)")
	confProxyPrefix := set.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	confGenreSplit := set.String("genre-split", "\n", "character or string to split genre tag data on (optional)")
	confHTTPLog := set.Bool("http-log", true, "http request logging (optional)")
	confShowVersion := set.Bool("version", false, "show gonic version")

	var confMusicPaths paths.MusicPaths
	set.Var(&confMusicPaths, "music-path", "path to music")

	_ = set.String("config-path", "", "path to config (optional)")

	if err := ff.Parse(set, os.Args[1:],
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix(gonic.NameUpper),
	); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}

	if *confShowVersion {
		fmt.Println(gonic.Version)
		os.Exit(0)
	}

	log.Printf("starting gonic %s\n", gonic.Version)
	log.Printf("provided config\n")
	set.VisitAll(func(f *flag.Flag) {
		value := strings.ReplaceAll(f.Value.String(), "\n", "")
		log.Printf("    %-25s %s\n", f.Name, value)
	})

	if len(confMusicPaths) == 0 {
		log.Fatalf("please provide a music directory")
	}
	for _, confMusicPath := range confMusicPaths {
		if _, err := os.Stat(confMusicPath.Path); os.IsNotExist(err) {
			log.Fatalf("music directory %q not found", confMusicPath.Path)
		}
	}
	if _, err := os.Stat(*confPodcastPath); os.IsNotExist(err) {
		log.Fatal("please provide a valid podcast directory")
	}

	if *confCachePath == "" {
		log.Fatal("please provide a cache directory")
	}

	cacheDirAudio := path.Join(*confCachePath, cachePrefixAudio)
	cacheDirCovers := path.Join(*confCachePath, cachePrefixCovers)
	if _, err := os.Stat(cacheDirAudio); os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDirAudio, os.ModePerm); err != nil {
			log.Fatalf("couldn't create audio cache path: %v\n", err)
		}
	}
	if _, err := os.Stat(cacheDirCovers); os.IsNotExist(err) {
		if err := os.MkdirAll(cacheDirCovers, os.ModePerm); err != nil {
			log.Fatalf("couldn't create covers cache path: %v\n", err)
		}
	}

	dbc, err := db.New(*confDBPath, db.DefaultOptions())
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer dbc.Close()

	err = dbc.Migrate(db.MigrationContext{
		OriginalMusicPath: confMusicPaths[0].Path,
	})
	if err != nil {
		log.Panicf("error migrating database: %v\n", err)
	}

	proxyPrefixExpr := regexp.MustCompile(`^\/*(.*?)\/*$`)
	*confProxyPrefix = proxyPrefixExpr.ReplaceAllString(*confProxyPrefix, `/$1`)
	server, err := server.New(server.Options{
		DB:             dbc,
		MusicPaths:     confMusicPaths,
		CachePath:      filepath.Clean(cacheDirAudio),
		CoverCachePath: cacheDirCovers,
		ProxyPrefix:    *confProxyPrefix,
		GenreSplit:     *confGenreSplit,
		PodcastPath:    filepath.Clean(*confPodcastPath),
		PlaylistPath:   *confPlaylistPath,
		HTTPLog:        *confHTTPLog,
		JukeboxEnabled: *confJukeboxEnabled,
	})
	if err != nil {
		log.Panicf("error creating server: %v\n", err)
	}

	var g run.Group
	g.Add(server.StartHTTP(*confListenAddr, *confTLSCert, *confTLSKey))
	g.Add(server.StartSessionClean(cleanTimeDuration))
	g.Add(server.StartPodcastRefresher(time.Hour))
	if *confScanIntervalMins > 0 {
		tickerDur := time.Duration(*confScanIntervalMins) * time.Minute
		g.Add(server.StartScanTicker(tickerDur))
	}
	if *confScanWatcher {
		g.Add(server.StartScanWatcher())
	}
	if *confJukeboxEnabled {
		extraArgs, _ := shlex.Split(*confJukeboxMPVExtraArgs)
		g.Add(server.StartJukebox(extraArgs))
	}
	if *confPodcastPurgeAgeDays > 0 {
		g.Add(server.StartPodcastPurger(time.Duration(*confPodcastPurgeAgeDays) * 24 * time.Hour))
	}
	if *confScanAtStart {
		server.ScanAtStart()
	}

	if err := g.Run(); err != nil {
		log.Panicf("error in job: %v", err)
	}
}
