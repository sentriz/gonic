// Package main is the gonic server entrypoint
//
//nolint:lll // flags help strings
package main

import (
	"errors"
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
	"go.senan.xyz/gonic/server"
	"go.senan.xyz/gonic/server/ctrlsubsonic"
)

const (
	cleanTimeDuration = 10 * time.Minute
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

	confDBPath := set.String("db-path", "gonic.db", "path to database (optional)")

	confScanIntervalMins := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	confScanAtStart := set.Bool("scan-at-start-enabled", false, "whether to perform an initial scan at startup (optional)")
	confScanWatcher := set.Bool("scan-watcher-enabled", false, "whether to watch file system for new music and rescan (optional)")

	confJukeboxEnabled := set.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
	confJukeboxMPVExtraArgs := set.String("jukebox-mpv-extra-args", "", "extra command line arguments to pass to the jukebox mpv daemon (optional)")

	confProxyPrefix := set.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	confHTTPLog := set.Bool("http-log", true, "http request logging (optional)")

	confGenreSplit := set.String("genre-split", "\n", "character or string to split genre tag data on (optional)")

	confShowVersion := set.Bool("version", false, "show gonic version")
	_ = set.String("config-path", "", "path to config (optional)")

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
	server, err := server.New(server.Options{
		DB:             dbc,
		MusicPaths:     musicPaths,
		CacheAudioPath: cacheDirAudio,
		CoverCachePath: cacheDirCovers,
		PodcastPath:    *confPodcastPath,
		ProxyPrefix:    *confProxyPrefix,
		GenreSplit:     *confGenreSplit,
		HTTPLog:        *confHTTPLog,
		JukeboxEnabled: *confJukeboxEnabled,
	})
	if err != nil {
		log.Panicf("error creating server: %v\n", err)
	}

	log.Printf("starting gonic v%s\n", gonic.Version)
	log.Printf("provided config\n")
	set.VisitAll(func(f *flag.Flag) {
		value := strings.ReplaceAll(f.Value.String(), "\n", "")
		log.Printf("    %-25s %s\n", f.Name, value)
	})

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

const pathAliasSep = "->"

type pathAliases []pathAlias
type pathAlias struct{ alias, path string }

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

var errNotExists = errors.New("path does not exist, please provide one")

func validatePath(p string) (string, error) {
	if p == "" {
		return "", errNotExists
	}
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return "", errNotExists
	}
	p, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("make absolute: %w", err)
	}
	return p, nil
}
