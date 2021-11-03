// Package main is the gonic server entrypoint
//nolint:lll // flags help strings
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff"

	"go.senan.xyz/gonic"
	"go.senan.xyz/gonic/server"
	"go.senan.xyz/gonic/server/db"
)

const (
	cleanTimeDuration = 10 * time.Minute
	cachePrefixAudio  = "audio"
	cachePrefixCovers = "covers"
)

func main() {
	set := flag.NewFlagSet(gonic.Name, flag.ExitOnError)
	confListenAddr := set.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")
	confPodcastPath := set.String("podcast-path", "", "path to podcasts")
	confCachePath := set.String("cache-path", "", "path to cache")
	confDBPath := set.String("db-path", "gonic.db", "path to database (optional)")
	confScanInterval := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	confJukeboxEnabled := set.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
	confProxyPrefix := set.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	confGenreSplit := set.String("genre-split", "\n", "character or string to split genre tag data on (optional)")
	confHTTPLog := set.Bool("http-log", true, "http request logging (optional)")
	confShowVersion := set.Bool("version", false, "show gonic version")

	var confMusicPaths musicPaths
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
		log.Printf("    %-15s %s\n", f.Name, value)
	})

	if len(confMusicPaths) == 0 {
		log.Fatalf("please provide a music directory")
	}
	for _, confMusicPath := range confMusicPaths {
		if _, err := os.Stat(confMusicPath); os.IsNotExist(err) {
			log.Fatalf("music directory %q not found", confMusicPath)
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
		OriginalMusicPath: confMusicPaths[0],
	})
	if err != nil {
		log.Panicf("error migrating database: %v\n", err)
	}

	proxyPrefixExpr := regexp.MustCompile(`^\/*(.*?)\/*$`)
	*confProxyPrefix = proxyPrefixExpr.ReplaceAllString(*confProxyPrefix, `/$1`)
	server, err := server.New(server.Options{
		DB:             dbc,
		MusicPaths:     confMusicPaths,
		CachePath:      cacheDirAudio,
		CoverCachePath: cacheDirCovers,
		ProxyPrefix:    *confProxyPrefix,
		GenreSplit:     *confGenreSplit,
		PodcastPath:    *confPodcastPath,
		HTTPLog:        *confHTTPLog,
		JukeboxEnabled: *confJukeboxEnabled,
	})
	if err != nil {
		log.Panicf("error creating server: %v\n", err)
	}

	var g run.Group
	g.Add(server.StartHTTP(*confListenAddr))
	g.Add(server.StartSessionClean(cleanTimeDuration))
	g.Add(server.StartPodcastRefresher(time.Hour))
	if *confScanInterval > 0 {
		tickerDur := time.Duration(*confScanInterval) * time.Minute
		g.Add(server.StartScanTicker(tickerDur))
	}
	if *confJukeboxEnabled {
		g.Add(server.StartJukebox())
	}

	if err := g.Run(); err != nil {
		log.Panicf("error in job: %v", err)
	}
}

type musicPaths []string

func (m musicPaths) String() string {
	return strings.Join(m, ", ")
}

func (m *musicPaths) Set(value string) error {
	*m = append(*m, value)
	return nil
}
