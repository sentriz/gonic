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
	"time"

	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/oklog/run"
	"github.com/peterbourgon/ff"

	"go.senan.xyz/gonic/server"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/version"
)

const (
	cleanTimeDuration = 10 * time.Minute
	coverCachePrefix  = "covers"
)

func main() {
	set := flag.NewFlagSet(version.NAME, flag.ExitOnError)
	confListenAddr := set.String("listen-addr", "0.0.0.0:4747", "listen address (optional)")
	confMusicPath := set.String("music-path", "", "path to music")
	confPodcastPath := set.String("podcast-path", "", "path to podcasts")
	confCachePath := set.String("cache-path", "/tmp/gonic_cache", "path to cache")
	confDBPath := set.String("db-path", "gonic.db", "path to database (optional)")
	confScanInterval := set.Int("scan-interval", 0, "interval (in minutes) to automatically scan music (optional)")
	confJukeboxEnabled := set.Bool("jukebox-enabled", false, "whether the subsonic jukebox api should be enabled (optional)")
	confProxyPrefix := set.String("proxy-prefix", "", "url path prefix to use if behind proxy. eg '/gonic' (optional)")
	confGenreSplit := set.String("genre-split", "\n", "character or string to split genre tag data on (optional)")
	confShowVersion := set.Bool("version", false, "show gonic version")
	_ = set.String("config-path", "", "path to config (optional)")

	if err := ff.Parse(set, os.Args[1:],
		ff.WithConfigFileFlag("config-path"),
		ff.WithConfigFileParser(ff.PlainParser),
		ff.WithEnvVarPrefix(version.NAME_UPPER),
	); err != nil {
		log.Fatalf("error parsing args: %v\n", err)
	}

	if *confShowVersion {
		fmt.Println(version.VERSION)
		os.Exit(0)
	}

	log.Printf("starting gonic %s\n", version.VERSION)
	log.Printf("provided config\n")
	set.VisitAll(func(f *flag.Flag) {
		log.Printf("    %-15s %s\n", f.Name, f.Value)
	})

	if _, err := os.Stat(*confMusicPath); os.IsNotExist(err) {
		log.Fatal("please provide a valid music directory")
	}
	if _, err := os.Stat(*confPodcastPath); *confPodcastPath != "" && os.IsNotExist(err) {
		log.Fatal("please provide a valid podcast directory")
	}
	if _, err := os.Stat(*confCachePath); os.IsNotExist(err) {
		if err := os.MkdirAll(*confCachePath, os.ModePerm); err != nil {
			log.Fatalf("couldn't create cache path: %v\n", err)
		}
	}
	coverCachePath := path.Join(*confCachePath, coverCachePrefix)
	if _, err := os.Stat(coverCachePath); os.IsNotExist(err) {
		if err := os.MkdirAll(coverCachePath, os.ModePerm); err != nil {
			log.Fatalf("couldn't create cover cache path: %v\n", err)
		}
	}

	db, err := db.New(*confDBPath)
	if err != nil {
		log.Fatalf("error opening database: %v\n", err)
	}
	defer db.Close()

	proxyPrefixExpr := regexp.MustCompile(`^\/*(.*?)\/*$`)
	*confProxyPrefix = proxyPrefixExpr.ReplaceAllString(*confProxyPrefix, `/$1`)
	server := server.New(server.Options{
		DB:             db,
		MusicPath:      *confMusicPath,
		CachePath:      *confCachePath,
		CoverCachePath: coverCachePath,
		ProxyPrefix:    *confProxyPrefix,
		GenreSplit:     *confGenreSplit,
		PodcastPath:    *confPodcastPath,
	})

	var g run.Group
	g.Add(server.StartHTTP(*confListenAddr))
	g.Add(server.StartSessionClean(cleanTimeDuration))
	if *confScanInterval > 0 {
		tickerDur := time.Duration(*confScanInterval) * time.Minute
		g.Add(server.StartScanTicker(tickerDur))
	}
	if *confJukeboxEnabled {
		g.Add(server.StartJukebox())
	}
	if *confProxyPrefix != "" {
		g.Add(server.StartPodcastRefresher(time.Hour))
	}

	if err := g.Run(); err != nil {
		log.Printf("error in job: %v", err)
	}
}
